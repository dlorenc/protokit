package discovery

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

func DoSpringCloudDiscoveryService(input string) ([]runtime.Object, error) {
	instance := &DiscoveryService{}
	if err := yaml.UnmarshalStrict([]byte(input), instance); err != nil {
		return nil, err
	}
	if len(instance.Generate.Selector) == 0 {
		return nil, fmt.Errorf("%s must specify SpringCloudDiscoveryService.Generate.Selector", instance.Name)
	}
	if len(instance.Generate.EurekaContainerName) == 0 && len(instance.Generate.Template.Spec.Containers) != 1 {
		return nil, fmt.Errorf("%s must specify SpringCloudDiscoveryService.Generate.EurekaContainerName", instance.Name)
	}
	if len(instance.Generate.Template.Spec.Containers) == 0 {
		return nil, fmt.Errorf("%s must specify a Container with the DiscoveryService container image", instance.Name)
	}

	statefulName := instance.Name
	identityServiceName := fmt.Sprintf("%s-identity", instance.Name)

	containers := instance.Generate.Template.Spec.Containers
	var eurekaContainer *corev1.Container
	if len(instance.Generate.EurekaContainerName) == 0 {
		eurekaContainer = &containers[0]
	} else {
		for i := range containers {
			if containers[i].Name == instance.Generate.EurekaContainerName {
				eurekaContainer = &containers[i]
			}
		}
	}
	if eurekaContainer == nil {
		return nil, fmt.Errorf("%s must specify SpringCloudDiscoveryService.EurekaContainerName %s not found", instance.Name, instance.Generate.EurekaContainerName)
	}

	identityService := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		Spec: corev1.ServiceSpec{
			Selector:  instance.Generate.Selector,
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:       "8761",
					Port:       8761,
					TargetPort: intstr.FromInt(8761),
				},
			},
		},
		ObjectMeta: instance.ObjectMeta,
	}
	identityService.Name = identityServiceName

	var options []string
	for k, v := range instance.Generate.Options {
		options = append(options, fmt.Sprintf("--%s=%s", k, v))
	}
	sort.Strings(options)

	// Calculate default zone
	var defaultZone = fmt.Sprintf("--eureka.client.serviceUrl.defaultZone")
	if instance.Generate.Replicas == nil {
		r := int32(1)
		instance.Generate.Replicas = &r
	}
	var zones []string
	for i := int32(0); i < *instance.Generate.Replicas; i++ {
		zones = append(zones, fmt.Sprintf("%s-%d.%s", statefulName, i, identityServiceName))
	}
	defaultZone = fmt.Sprintf("%s=%s", defaultZone, strings.Join(zones, ","))

	eurekaContainer.Command = []string{"java"}
	eurekaContainer.Args = []string{
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+UseCGroupMemoryLimitForHeap",
		"-Djava.security.egd=file:/dev/./urandom",
		"-jar", "/app.jar",
	}
	eurekaContainer.Args = append(eurekaContainer.Args, "--server.port=8761")
	eurekaContainer.Args = append(eurekaContainer.Args, defaultZone)
	eurekaContainer.Args = append(eurekaContainer.Args, fmt.Sprintf("--eureka.instance.hostname=$(POD_NAME).%s", identityService.Name))
	eurekaContainer.Args = append(eurekaContainer.Args, options...)

	eurekaContainer.Env = []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}

	eurekaContainer.ReadinessProbe = &corev1.Probe{
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      1,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(8761),
				Path: "/",
			},
		},
	}

	if len(instance.Generate.ConfigMapEnvName) > 0 {
		eurekaContainer.EnvFrom = append(eurekaContainer.EnvFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: instance.Generate.ConfigMapEnvName,
				},
			},
		})
	}

	if len(instance.Generate.SecretEnvName) > 0 {
		eurekaContainer.EnvFrom = append(eurekaContainer.EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: instance.Generate.SecretEnvName,
				},
			},
		})
	}

	stateful := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: instance.ObjectMeta,
		Spec: appsv1.StatefulSetSpec{
			ServiceName: identityService.Name,
			Replicas:    instance.Generate.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: instance.Generate.Selector,
			},
			Template: instance.Generate.Template,
		},
	}
	stateful.Name = statefulName

	lbService := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: instance.ObjectMeta,
		Spec: corev1.ServiceSpec{
			Selector: instance.Generate.Selector,
			Ports: []corev1.ServicePort{
				{
					Name:       "8761",
					Port:       8761,
					TargetPort: intstr.FromInt(8761),
				},
			},
		},
	}
	lbService.Name = fmt.Sprintf("%s-discovery", lbService.Name)

	return []runtime.Object{stateful, identityService, lbService}, nil
}

type DiscoveryService struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Generate Generate `json:"generate,omitempty"`
}

type Generate struct {
	Replicas *int32 `json:"replicas,omitempty"`

	EurekaContainerName string `json:"eurekaContainerName,omitempty"`
	ConfigMapEnvName    string `json:"configMapEnvName,omitempty"`
	SecretEnvName       string `json:"secretEnvName,omitempty"`

	Template corev1.PodTemplateSpec `json:"template,omitempty"`
	Options  map[string]string      `json:"options"`
	Selector map[string]string      `json:"selector,omitempty"`
}
