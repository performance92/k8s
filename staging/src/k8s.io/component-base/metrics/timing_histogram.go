/*
Copyright 2019 The Kubernetes Authors.

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

package metrics

import (
	"context"
	"time"

	"github.com/blang/semver/v4"
	promext "k8s.io/component-base/metrics/prometheusextension"
)

// TimingHistogram is our internal representation for our wrapping struct around timing
// histograms. It implements both kubeCollector and GaugeMetric
type TimingHistogram struct {
	GaugeMetric
	*TimingHistogramOpts
	nowFunc func() time.Time
	lazyMetric
	selfCollector
}

// NewTimingHistogram returns an object which is TimingHistogram-like. However, nothing
// will be measured until the histogram is registered somewhere.
func NewTimingHistogram(opts *TimingHistogramOpts) *TimingHistogram {
	return NewTestableTimingHistogram(time.Now, opts)
}

// NewTestableTimingHistogram adds injection of the clock
func NewTestableTimingHistogram(nowFunc func() time.Time, opts *TimingHistogramOpts) *TimingHistogram {
	opts.StabilityLevel.setDefaults()

	h := &TimingHistogram{
		TimingHistogramOpts: opts,
		nowFunc:             nowFunc,
		lazyMetric:          lazyMetric{},
	}
	h.setPrometheusHistogram(noopMetric{})
	h.lazyInit(h, BuildFQName(opts.Namespace, opts.Subsystem, opts.Name))
	return h
}

// setPrometheusHistogram sets the underlying KubeGauge object, i.e. the thing that does the measurement.
func (h *TimingHistogram) setPrometheusHistogram(histogram promext.TimingHistogram) {
	h.GaugeMetric = histogram
	h.initSelfCollection(histogram)
}

// DeprecatedVersion returns a pointer to the Version or nil
func (h *TimingHistogram) DeprecatedVersion() *semver.Version {
	return parseSemver(h.TimingHistogramOpts.DeprecatedVersion)
}

// initializeMetric invokes the actual prometheus.Histogram object instantiation
// and stores a reference to it
func (h *TimingHistogram) initializeMetric() {
	h.TimingHistogramOpts.annotateStabilityLevel()
	// this actually creates the underlying prometheus gauge.
	histogram, err := promext.NewTestableTimingHistogram(h.nowFunc, h.TimingHistogramOpts.toPromHistogramOpts())
	if err != nil {
		panic(err) // handle as for regular histograms
	}
	h.setPrometheusHistogram(histogram)
}

// initializeDeprecatedMetric invokes the actual prometheus.Histogram object instantiation
// but modifies the Help description prior to object instantiation.
func (h *TimingHistogram) initializeDeprecatedMetric() {
	h.TimingHistogramOpts.markDeprecated()
	h.initializeMetric()
}

// WithContext allows the normal TimingHistogram metric to pass in context. The context is no-op now.
func (h *TimingHistogram) WithContext(ctx context.Context) GaugeMetric {
	return h.GaugeMetric
}

// timingHistogramVec is the internal representation of our wrapping struct around prometheus
// TimingHistogramVecs.
type timingHistogramVec struct {
	*promext.TimingHistogramVec
	*TimingHistogramOpts
	nowFunc func() time.Time
	lazyMetric
	originalLabels []string
}

// NewTimingHistogramVec returns an object which satisfies kubeCollector and
// wraps the promext.timingHistogramVec object.  Note well the way that
// behavior depends on registration and whether this is hidden.
func NewTimingHistogramVec(opts *TimingHistogramOpts, labels []string) PreContextAndRegisterableGaugeMetricVec {
	return NewTestableTimingHistogramVec(time.Now, opts, labels)
}

// NewTestableTimingHistogramVec adds injection of the clock.
func NewTestableTimingHistogramVec(nowFunc func() time.Time, opts *TimingHistogramOpts, labels []string) PreContextAndRegisterableGaugeMetricVec {
	opts.StabilityLevel.setDefaults()

	fqName := BuildFQName(opts.Namespace, opts.Subsystem, opts.Name)
	allowListLock.RLock()
	if allowList, ok := labelValueAllowLists[fqName]; ok {
		opts.LabelValueAllowLists = allowList
	}
	allowListLock.RUnlock()

	v := &timingHistogramVec{
		TimingHistogramVec:  noopTimingHistogramVec,
		TimingHistogramOpts: opts,
		nowFunc:             nowFunc,
		originalLabels:      labels,
		lazyMetric:          lazyMetric{},
	}
	v.lazyInit(v, fqName)
	return v
}

// DeprecatedVersion returns a pointer to the Version or nil
func (v *timingHistogramVec) DeprecatedVersion() *semver.Version {
	return parseSemver(v.TimingHistogramOpts.DeprecatedVersion)
}

func (v *timingHistogramVec) initializeMetric() {
	v.TimingHistogramOpts.annotateStabilityLevel()
	v.TimingHistogramVec = promext.NewTestableTimingHistogramVec(v.nowFunc, v.TimingHistogramOpts.toPromHistogramOpts(), v.originalLabels...)
}

func (v *timingHistogramVec) initializeDeprecatedMetric() {
	v.TimingHistogramOpts.markDeprecated()
	v.initializeMetric()
}

// WithLabelValuesChecked, if called on a hidden vector,
// will return a noop gauge and a nil error.
// If called before this vector has been registered in
// at least one registry, will return a noop gauge and
// an error that passes ErrIsNotRegistered.
// If called with a syntactic problem in the labels, will
// return a noop gauge and an error about the labels.
// If none of the above apply, this method will return
// the appropriate vector member and a nil error.
func (v *timingHistogramVec) WithLabelValuesChecked(lvs ...string) (GaugeMetric, error) {
	if v.IsHidden() {
		return noop, nil
	}
	if !v.IsCreated() {
		return noop, errNotRegistered
	}
	if v.LabelValueAllowLists != nil {
		v.LabelValueAllowLists.ConstrainToAllowedList(v.originalLabels, lvs)
	}
	ops, err := v.TimingHistogramVec.GetMetricWithLabelValues(lvs...)
	return ops.(GaugeMetric), err
}

// WithLabelValues calls WithLabelValuesChecked
// and handles errors as follows.
// An error that passes ErrIsNotRegistered is ignored
// and the noop gauge is returned;
// all other errors cause a panic.
func (v *timingHistogramVec) WithLabelValues(lvs ...string) GaugeMetric {
	ans, err := v.WithLabelValuesChecked(lvs...)
	if err == nil || ErrIsNotRegistered(err) {
		return ans
	}
	panic(err)
}

// WithChecked, if called on a hidden vector,
// will return a noop gauge and a nil error.
// If called before this vector has been registered in
// at least one registry, will return a noop gauge and
// an error that passes ErrIsNotRegistered.
// If called with a syntactic problem in the labels, will
// return a noop gauge and an error about the labels.
// If none of the above apply, this method will return
// the appropriate vector member and a nil error.
func (v *timingHistogramVec) WithChecked(labels map[string]string) (GaugeMetric, error) {
	if v.IsHidden() {
		return noop, nil
	}
	if !v.IsCreated() {
		return noop, errNotRegistered
	}
	if v.LabelValueAllowLists != nil {
		v.LabelValueAllowLists.ConstrainLabelMap(labels)
	}
	ops, err := v.TimingHistogramVec.GetMetricWith(labels)
	return ops.(GaugeMetric), err
}

// With calls WithChecked and handles errors as follows.
// An error that passes ErrIsNotRegistered is ignored
// and the noop gauge is returned;
// all other errors cause a panic.
func (v *timingHistogramVec) With(labels map[string]string) GaugeMetric {
	ans, err := v.WithChecked(labels)
	if err == nil || ErrIsNotRegistered(err) {
		return ans
	}
	panic(err)
}

// Delete deletes the metric where the variable labels are the same as those
// passed in as labels. It returns true if a metric was deleted.
//
// It is not an error if the number and names of the Labels are inconsistent
// with those of the VariableLabels in Desc. However, such inconsistent Labels
// can never match an actual metric, so the method will always return false in
// that case.
func (v *timingHistogramVec) Delete(labels map[string]string) bool {
	if !v.IsCreated() {
		return false // since we haven't created the metric, we haven't deleted a metric with the passed in values
	}
	return v.TimingHistogramVec.Delete(labels)
}

// Reset deletes all metrics in this vector.
func (v *timingHistogramVec) Reset() {
	if !v.IsCreated() {
		return
	}

	v.TimingHistogramVec.Reset()
}

// WithContext returns wrapped timingHistogramVec with context
func (v *timingHistogramVec) WithContext(ctx context.Context) GaugeMetricVec {
	return &TimingHistogramVecWithContext{
		ctx:                ctx,
		timingHistogramVec: v,
	}
}

// TimingHistogramVecWithContext is the wrapper of timingHistogramVec with context.
// Currently the context is ignored.
type TimingHistogramVecWithContext struct {
	*timingHistogramVec
	ctx context.Context
}
