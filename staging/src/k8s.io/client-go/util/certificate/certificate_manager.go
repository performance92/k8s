/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	certificates "k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	certificatesclient "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
)

// certificateWaitBackoff controls the amount and timing of retries when the
// watch for certificate approval is interrupted.
var certificateWaitBackoff = wait.Backoff{Duration: 30 * time.Second, Steps: 4, Factor: 1.5, Jitter: 0.1}

// Manager maintains and updates the certificates in use by this certificate
// manager. In the background it communicates with the API server to get new
// certificates for certificates about to expire.
type Manager interface {
	// CertificateSigningRequestClient sets the client interface that is used for
	// signing new certificates generated as part of rotation.
	SetCertificateSigningRequestClient(certificatesclient.CertificateSigningRequestInterface) error
	// Start the API server status sync loop.
	Start()
	// Current returns the currently selected certificate from the
	// certificate manager, as well as the associated certificate and key data
	// in PEM format.
	Current() *tls.Certificate
	// ServerHealthy returns true if the manager is able to communicate with
	// the server. This allows a caller to determine whether the cert manager
	// thinks it can potentially talk to the API server. The cert manager may
	// be very conservative and only return true if recent communication has
	// occurred with the server.
	ServerHealthy() bool
}

// Config is the set of configuration parameters available for a new Manager.
type Config struct {
	// CertificateSigningRequestClient will be used for signing new certificate
	// requests generated when a key rotation occurs. It must be set either at
	// initialization or by using CertificateSigningRequestClient before
	// Manager.Start() is called.
	CertificateSigningRequestClient certificatesclient.CertificateSigningRequestInterface
	// Template is the CertificateRequest that will be used as a template for
	// generating certificate signing requests for all new keys generated as
	// part of rotation. It follows the same rules as the template parameter of
	// crypto.x509.CreateCertificateRequest in the Go standard libraries.
	Template *x509.CertificateRequest
	// Usages is the types of usages that certificates generated by the manager
	// can be used for.
	Usages []certificates.KeyUsage
	// CertificateStore is a persistent store where the current cert/key is
	// kept and future cert/key pairs will be persisted after they are
	// generated.
	CertificateStore Store
	// BootstrapCertificatePEM is the certificate data that will be returned
	// from the Manager if the CertificateStore doesn't have any cert/key pairs
	// currently available and has not yet had a chance to get a new cert/key
	// pair from the API. If the CertificateStore does have a cert/key pair,
	// this will be ignored. If there is no cert/key pair available in the
	// CertificateStore, as soon as Start is called, it will request a new
	// cert/key pair from the CertificateSigningRequestClient. This is intended
	// to allow the first boot of a component to be initialized using a
	// generic, multi-use cert/key pair which will be quickly replaced with a
	// unique cert/key pair.
	BootstrapCertificatePEM []byte
	// BootstrapKeyPEM is the key data that will be returned from the Manager
	// if the CertificateStore doesn't have any cert/key pairs currently
	// available. If the CertificateStore does have a cert/key pair, this will
	// be ignored. If the bootstrap cert/key pair are used, they will be
	// rotated at the first opportunity, possibly well in advance of expiring.
	// This is intended to allow the first boot of a component to be
	// initialized using a generic, multi-use cert/key pair which will be
	// quickly replaced with a unique cert/key pair.
	BootstrapKeyPEM []byte
	// CertificateExpiration will record a metric that shows the remaining
	// lifetime of the certificate.
	CertificateExpiration Gauge

	// CertBeginRotationFraction determines how far into the life of a
	// certificate the kubelet will wait before it begins attempting to rotate the
	// certificate. Default value is 0.8.
	CertBeginRotationFraction float64
}

// Store is responsible for getting and updating the current certificate.
// Depending on the concrete implementation, the backing store for this
// behavior may vary.
type Store interface {
	// Current returns the currently selected certificate, as well as the
	// associated certificate and key data in PEM format. If the Store doesn't
	// have a cert/key pair currently, it should return a NoCertKeyError so
	// that the Manager can recover by using bootstrap certificates to request
	// a new cert/key pair.
	Current() (*tls.Certificate, error)
	// Update accepts the PEM data for the cert/key pair and makes the new
	// cert/key pair the 'current' pair, that will be returned by future calls
	// to Current().
	Update(cert, key []byte) (*tls.Certificate, error)
}

// Gauge will record the remaining lifetime of the certificate each time it is
// updated.
type Gauge interface {
	Set(float64)
}

// NoCertKeyError indicates there is no cert/key currently available.
type NoCertKeyError string

func (e *NoCertKeyError) Error() string { return string(*e) }

type manager struct {
	certSigningRequestClient  certificatesclient.CertificateSigningRequestInterface
	template                  *x509.CertificateRequest
	usages                    []certificates.KeyUsage
	certStore                 Store
	certAccessLock            sync.RWMutex
	cert                      *tls.Certificate
	rotationDeadline          time.Time
	forceRotation             bool
	certificateExpiration     Gauge
	serverHealth              bool
	certBeginRotationFraction float64
}

// NewManager returns a new certificate manager. A certificate manager is
// responsible for being the authoritative source of certificates in the
// Kubelet and handling updates due to rotation.
func NewManager(config *Config) (Manager, error) {
	cert, forceRotation, err := getCurrentCertificateOrBootstrap(
		config.CertificateStore,
		config.BootstrapCertificatePEM,
		config.BootstrapKeyPEM)
	if err != nil {
		return nil, err
	}

	m := manager{
		certSigningRequestClient:  config.CertificateSigningRequestClient,
		template:                  config.Template,
		usages:                    config.Usages,
		certStore:                 config.CertificateStore,
		cert:                      cert,
		forceRotation:             forceRotation,
		certificateExpiration:     config.CertificateExpiration,
		certBeginRotationFraction: config.CertBeginRotationFraction,
	}
	return &m, nil
}

// Current returns the currently selected certificate from the certificate
// manager. This can be nil if the manager was initialized without a
// certificate and has not yet received one from the
// CertificateSigningRequestClient.
func (m *manager) Current() *tls.Certificate {
	m.certAccessLock.RLock()
	defer m.certAccessLock.RUnlock()
	return m.cert
}

// ServerHealthy returns true if the cert manager believes the server
// is currently alive.
func (m *manager) ServerHealthy() bool {
	m.certAccessLock.RLock()
	defer m.certAccessLock.RUnlock()
	return m.serverHealth
}

// SetCertificateSigningRequestClient sets the client interface that is used
// for signing new certificates generated as part of rotation. It must be
// called before Start() and can not be used to change the
// CertificateSigningRequestClient that has already been set. This method is to
// support the one specific scenario where the CertificateSigningRequestClient
// uses the CertificateManager.
func (m *manager) SetCertificateSigningRequestClient(certSigningRequestClient certificatesclient.CertificateSigningRequestInterface) error {
	if m.certSigningRequestClient == nil {
		m.certSigningRequestClient = certSigningRequestClient
		return nil
	}
	return fmt.Errorf("property CertificateSigningRequestClient is already set")
}

// Start will start the background work of rotating the certificates.
func (m *manager) Start() {
	// Certificate rotation depends on access to the API server certificate
	// signing API, so don't start the certificate manager if we don't have a
	// client. This will happen on the cluster master, where the kubelet is
	// responsible for bootstrapping the pods of the master components.
	if m.certSigningRequestClient == nil {
		glog.V(2).Infof("Certificate rotation is not enabled, no connection to the apiserver.")
		return
	}

	glog.V(2).Infof("Certificate rotation is enabled.")

	m.setRotationDeadline()

	// Synchronously request a certificate before entering the background
	// loop to allow bootstrap scenarios, where the certificate manager
	// doesn't have a certificate at all yet.
	if m.shouldRotate() {
		glog.V(1).Infof("shouldRotate() is true, forcing immediate rotation")
		if _, err := m.rotateCerts(); err != nil {
			utilruntime.HandleError(fmt.Errorf("Could not rotate certificates: %v", err))
		}
	}
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Jitter:   0.1,
		Steps:    5,
	}
	go wait.Forever(func() {
		sleepInterval := m.rotationDeadline.Sub(time.Now())
		glog.V(2).Infof("Waiting %v for next certificate rotation", sleepInterval)
		time.Sleep(sleepInterval)
		if err := wait.ExponentialBackoff(backoff, m.rotateCerts); err != nil {
			utilruntime.HandleError(fmt.Errorf("Reached backoff limit, still unable to rotate certs: %v", err))
			wait.PollInfinite(32*time.Second, m.rotateCerts)
		}
	}, 0)
}

func getCurrentCertificateOrBootstrap(
	store Store,
	bootstrapCertificatePEM []byte,
	bootstrapKeyPEM []byte) (cert *tls.Certificate, shouldRotate bool, errResult error) {

	currentCert, err := store.Current()
	if err == nil {
		return currentCert, false, nil
	}

	if _, ok := err.(*NoCertKeyError); !ok {
		return nil, false, err
	}

	if bootstrapCertificatePEM == nil || bootstrapKeyPEM == nil {
		return nil, true, nil
	}

	bootstrapCert, err := tls.X509KeyPair(bootstrapCertificatePEM, bootstrapKeyPEM)
	if err != nil {
		return nil, false, err
	}
	if len(bootstrapCert.Certificate) < 1 {
		return nil, false, fmt.Errorf("no cert/key data found")
	}

	certs, err := x509.ParseCertificates(bootstrapCert.Certificate[0])
	if err != nil {
		return nil, false, fmt.Errorf("unable to parse certificate data: %v", err)
	}
	bootstrapCert.Leaf = certs[0]
	return &bootstrapCert, true, nil
}

// shouldRotate looks at how close the current certificate is to expiring and
// decides if it is time to rotate or not.
func (m *manager) shouldRotate() bool {
	m.certAccessLock.RLock()
	defer m.certAccessLock.RUnlock()
	if m.cert == nil {
		return true
	}
	if m.forceRotation {
		return true
	}
	return time.Now().After(m.rotationDeadline)
}

// rotateCerts attempts to request a client cert from the server, wait a reasonable
// period of time for it to be signed, and then update the cert on disk. If it cannot
// retrieve a cert, it will return false. It will only return error in exceptional cases.
// This method also keeps track of "server health" by interpreting the responses it gets
// from the server on the various calls it makes.
func (m *manager) rotateCerts() (bool, error) {
	glog.V(2).Infof("Rotating certificates")

	csrPEM, keyPEM, privateKey, err := m.generateCSR()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to generate a certificate signing request: %v", err))
		return false, nil
	}

	// Call the Certificate Signing Request API to get a certificate for the
	// new private key.
	req, err := csr.RequestCertificate(m.certSigningRequestClient, csrPEM, "", m.usages, privateKey)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed while requesting a signed certificate from the master: %v", err))
		return false, m.updateServerError(err)
	}

	// Wait for the certificate to be signed. Instead of one long watch, we retry with slighly longer
	// intervals each time in order to tolerate failures from the server AND to preserve the liveliness
	// of the cert manager loop. This creates slightly more traffic against the API server in return
	// for bounding the amount of time we wait when a certificate expires.
	var crtPEM []byte
	watchDuration := time.Minute
	if err := wait.ExponentialBackoff(certificateWaitBackoff, func() (bool, error) {
		data, err := csr.WaitForCertificate(m.certSigningRequestClient, req, watchDuration)
		switch {
		case err == nil:
			crtPEM = data
			return true, nil
		case err == wait.ErrWaitTimeout:
			watchDuration += time.Minute
			if watchDuration > 5*time.Minute {
				watchDuration = 5 * time.Minute
			}
			return false, nil
		default:
			utilruntime.HandleError(fmt.Errorf("Unable to check certificate signing status: %v", err))
			return false, m.updateServerError(err)
		}
	}); err != nil {
		utilruntime.HandleError(fmt.Errorf("Certificate request was not signed: %v", err))
		return false, nil
	}

	cert, err := m.certStore.Update(crtPEM, keyPEM)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to store the new cert/key pair: %v", err))
		return false, nil
	}

	m.updateCached(cert)
	m.setRotationDeadline()
	m.forceRotation = false
	return true, nil
}

// setRotationDeadline sets a cached value for the threshold at which the
// current certificate should be rotated, 80%+/-10% of the expiration of the
// certificate.
func (m *manager) setRotationDeadline() {
	m.certAccessLock.RLock()
	defer m.certAccessLock.RUnlock()
	if m.cert == nil {
		m.rotationDeadline = time.Now()
		return
	}

	notAfter := m.cert.Leaf.NotAfter
	totalDuration := float64(notAfter.Sub(m.cert.Leaf.NotBefore))

	m.rotationDeadline = m.cert.Leaf.NotBefore.Add(jitteryDuration(totalDuration * m.certBeginRotationFraction))
	glog.V(2).Infof("Certificate expiration is %v, rotation deadline is %v", notAfter, m.rotationDeadline)
	if m.certificateExpiration != nil {
		m.certificateExpiration.Set(float64(notAfter.Unix()))
	}
}

// Returns a random time within totalDuration +- 5%.
// Jitter is added so that if a number of nodes are added to a cluster at
// approximately the same time (such as cluster creation time), they won't all
// try to rotate certificates at the same time for the rest of the life of the
// cluster.
//
// This function is represented as a variable to allow replacement during testing.
var jitteryDuration = func(totalDuration float64) time.Duration {
	const JitterFactor = 0.1
	return wait.Jitter(time.Duration(totalDuration), JitterFactor) - time.Duration((totalDuration * JitterFactor / 2))
}

// updateCached sets the most recent retrieved cert. It also sets the server
// as assumed healthy.
func (m *manager) updateCached(cert *tls.Certificate) {
	m.certAccessLock.Lock()
	defer m.certAccessLock.Unlock()
	m.serverHealth = true
	m.cert = cert
}

// updateServerError takes an error returned by the server and infers
// the health of the server based on the error. It will return nil if
// the error does not require immediate termination of any wait loops,
// and otherwise it will return the error.
func (m *manager) updateServerError(err error) error {
	m.certAccessLock.Lock()
	defer m.certAccessLock.Unlock()
	switch {
	case errors.IsUnauthorized(err):
		// SSL terminating proxies may report this error instead of the master
		m.serverHealth = true
	case errors.IsUnexpectedServerError(err):
		// generally indicates a proxy or other load balancer problem, rather than a problem coming
		// from the master
		m.serverHealth = false
	default:
		// Identify known errors that could be expected for a cert request that
		// indicate everything is working normally
		m.serverHealth = errors.IsNotFound(err) || errors.IsForbidden(err)
	}
	return nil
}

func (m *manager) generateCSR() (csrPEM []byte, keyPEM []byte, key interface{}, err error) {
	// Generate a new private key.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to generate a new private key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to marshal the new key to DER: %v", err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{Type: cert.ECPrivateKeyBlockType, Bytes: der})

	csrPEM, err = cert.MakeCSRFromTemplate(privateKey, m.template)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unable to create a csr from the private key: %v", err)
	}
	return csrPEM, keyPEM, privateKey, nil
}
