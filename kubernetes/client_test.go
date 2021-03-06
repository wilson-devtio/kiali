package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterDeploymentsForService(t *testing.T) {
	assert := assert.New(t)

	service := v1.Service{
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"foo": "bar"}}}

	pods := v1.PodList{
		Items: []v1.Pod{
			v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:   "httpbin-v1",
					Labels: map[string]string{"foo": "bazz", "version": "v1"}}},
			v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:   "reviews-v1",
					Labels: map[string]string{"foo": "bar", "version": "v1"}}},
			v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:   "reviews-v2",
					Labels: map[string]string{"foo": "bar", "version": "v2"}}},
		}}

	deployments := v1beta1.DeploymentList{
		Items: []v1beta1.Deployment{
			v1beta1.Deployment{
				ObjectMeta: meta_v1.ObjectMeta{Name: "reviews-v1"},
				Spec: v1beta1.DeploymentSpec{
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar", "version": "v1"}}}},
			v1beta1.Deployment{
				ObjectMeta: meta_v1.ObjectMeta{Name: "reviews-v2"},
				Spec: v1beta1.DeploymentSpec{
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar", "version": "v2"}}}},
			v1beta1.Deployment{
				ObjectMeta: meta_v1.ObjectMeta{Name: "httpbin-v1"},
				Spec: v1beta1.DeploymentSpec{
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bazz", "version": "v1"}}}},
		}}

	matches := FilterDeploymentsForService(&service, &pods, &deployments)

	assert.Len(matches, 2)
	assert.Equal("reviews-v1", matches[0].ObjectMeta.Name)
	assert.Equal("reviews-v2", matches[1].ObjectMeta.Name)
}

func TestFilterDeploymentsForServiceWithSpecificLabels(t *testing.T) {
	assert := assert.New(t)

	service := v1.Service{
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"jaeger-infra": "jaeger-pod"}}}

	pods := v1.PodList{
		Items: []v1.Pod{
			v1.Pod{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:   "jaeger-pod",
					Labels: map[string]string{"jaeger-infra": "jaeger-pod", "hash": "123456"}}},
		}}

	deployments := v1beta1.DeploymentList{
		Items: []v1beta1.Deployment{
			v1beta1.Deployment{
				Spec: v1beta1.DeploymentSpec{
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{"jaeger-infra": "jaeger-pod"}}}},
		}}

	matches := FilterDeploymentsForService(&service, &pods, &deployments)
	assert.Len(matches, 1)
}

func TestFilterDeploymentsForServiceWithoutPod(t *testing.T) {
	assert := assert.New(t)

	service := v1.Service{
		Spec: v1.ServiceSpec{
			Selector: map[string]string{"app": "foo"}}}

	pods := v1.PodList{}

	deployments := v1beta1.DeploymentList{
		Items: []v1beta1.Deployment{
			v1beta1.Deployment{
				ObjectMeta: meta_v1.ObjectMeta{
					Labels: map[string]string{"app": "foo", "hash": "123456"}}},
		}}

	matches := FilterDeploymentsForService(&service, &pods, &deployments)
	assert.Len(matches, 1)
}
