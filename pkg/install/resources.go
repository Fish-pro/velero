/*
Copyright the Velero contributors.

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

package install

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1crds "github.com/vmware-tanzu/velero/config/crd/v1/crds"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

const defaultServiceAccountName = "velero"

var (
	DefaultVeleroPodCPURequest    = "500m"
	DefaultVeleroPodMemRequest    = "128Mi"
	DefaultVeleroPodCPULimit      = "1000m"
	DefaultVeleroPodMemLimit      = "512Mi"
	DefaultNodeAgentPodCPURequest = "500m"
	DefaultNodeAgentPodMemRequest = "512Mi"
	DefaultNodeAgentPodCPULimit   = "1000m"
	DefaultNodeAgentPodMemLimit   = "1Gi"
	DefaultVeleroNamespace        = "velero"
)

func Labels() map[string]string {
	return map[string]string{
		"component": "velero",
	}
}

func podLabels(userLabels ...map[string]string) map[string]string {
	// Use the default labels as a starting point
	base := Labels()

	// Merge base labels with user labels to enforce CLI precedence
	for _, labels := range userLabels {
		for k, v := range labels {
			base[k] = v
		}
	}

	return base
}

func podAnnotations(userAnnotations map[string]string) map[string]string {
	// Use the default annotations as a starting point
	base := map[string]string{
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "8085",
		"prometheus.io/path":   "/metrics",
	}

	// Merge base annotations with user annotations to enforce CLI precedence
	for k, v := range userAnnotations {
		base[k] = v
	}

	return base
}

func containerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "metrics",
			ContainerPort: 8085,
		},
	}
}

func objectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    Labels(),
	}
}

func ServiceAccount(namespace string, annotations map[string]string) *corev1.ServiceAccount {
	objMeta := objectMeta(namespace, defaultServiceAccountName)
	objMeta.Annotations = annotations
	return &corev1.ServiceAccount{
		ObjectMeta: objMeta,
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
}

func ClusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	crbName := "velero"
	if namespace != DefaultVeleroNamespace {
		crbName = "velero-" + namespace
	}
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: objectMeta("", crbName),
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      "velero",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return crb
}

func Namespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: objectMeta("", namespace),
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
}

func BackupStorageLocation(namespace, provider, bucket, prefix string, config map[string]string, caCert []byte) *velerov1api.BackupStorageLocation {
	return &velerov1api.BackupStorageLocation{
		ObjectMeta: objectMeta(namespace, "default"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "BackupStorageLocation",
			APIVersion: velerov1api.SchemeGroupVersion.String(),
		},
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: provider,
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: bucket,
					Prefix: prefix,
					CACert: caCert,
				},
			},
			Config:  config,
			Default: true,
		},
	}
}

func VolumeSnapshotLocation(namespace, provider string, config map[string]string) *velerov1api.VolumeSnapshotLocation {
	return &velerov1api.VolumeSnapshotLocation{
		ObjectMeta: objectMeta(namespace, "default"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "VolumeSnapshotLocation",
			APIVersion: velerov1api.SchemeGroupVersion.String(),
		},
		Spec: velerov1api.VolumeSnapshotLocationSpec{
			Provider: provider,
			Config:   config,
		},
	}
}

func Secret(namespace string, data []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: objectMeta(namespace, "cloud-credentials"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Data: map[string][]byte{
			"cloud": data,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func appendUnstructured(list *unstructured.UnstructuredList, obj runtime.Object) error {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)

	// Remove the status field so we're not sending blank data to the server.
	// On CRDs, having an empty status is actually a validation error.
	delete(u, "status")
	if err != nil {
		return err
	}
	list.Items = append(list.Items, unstructured.Unstructured{Object: u})
	return nil
}

type VeleroOptions struct {
	Namespace                       string
	Image                           string
	ProviderName                    string
	Bucket                          string
	Prefix                          string
	PodAnnotations                  map[string]string
	PodLabels                       map[string]string
	ServiceAccountAnnotations       map[string]string
	ServiceAccountName              string
	VeleroPodResources              corev1.ResourceRequirements
	NodeAgentPodResources           corev1.ResourceRequirements
	SecretData                      []byte
	RestoreOnly                     bool
	UseNodeAgent                    bool
	UseVolumeSnapshots              bool
	BSLConfig                       map[string]string
	VSLConfig                       map[string]string
	DefaultRepoMaintenanceFrequency time.Duration
	GarbageCollectionFrequency      time.Duration
	Plugins                         []string
	NoDefaultBackupLocation         bool
	CACertData                      []byte
	Features                        []string
	DefaultVolumesToFsBackup        bool
	UploaderType                    string
}

func AllCRDs() *unstructured.UnstructuredList {
	resources := new(unstructured.UnstructuredList)
	// Set the GVK so that the serialization framework outputs the list properly
	resources.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "List"})

	for _, crd := range v1crds.CRDs {
		crd.SetLabels(Labels())
		appendUnstructured(resources, crd)
	}

	return resources
}

// AllResources returns a list of all resources necessary to install Velero, in the appropriate order, into a Kubernetes cluster.
// Items are unstructured, since there are different data types returned.
func AllResources(o *VeleroOptions) *unstructured.UnstructuredList {
	resources := AllCRDs()

	ns := Namespace(o.Namespace)
	appendUnstructured(resources, ns)

	crb := ClusterRoleBinding(o.Namespace)
	appendUnstructured(resources, crb)
	serviceAccountName := defaultServiceAccountName
	if o.ServiceAccountName == "" {
		sa := ServiceAccount(o.Namespace, o.ServiceAccountAnnotations)
		appendUnstructured(resources, sa)
	} else {
		serviceAccountName = o.ServiceAccountName
	}

	if o.SecretData != nil {
		sec := Secret(o.Namespace, o.SecretData)
		appendUnstructured(resources, sec)
	}

	if !o.NoDefaultBackupLocation {
		bsl := BackupStorageLocation(o.Namespace, o.ProviderName, o.Bucket, o.Prefix, o.BSLConfig, o.CACertData)
		appendUnstructured(resources, bsl)
	}

	// A snapshot location may not be desirable for users relying on pod volume backup/restore
	if o.UseVolumeSnapshots {
		vsl := VolumeSnapshotLocation(o.Namespace, o.ProviderName, o.VSLConfig)
		appendUnstructured(resources, vsl)
	}

	secretPresent := o.SecretData != nil

	deployOpts := []podTemplateOption{
		WithAnnotations(o.PodAnnotations),
		WithLabels(o.PodLabels),
		WithImage(o.Image),
		WithResources(o.VeleroPodResources),
		WithSecret(secretPresent),
		WithDefaultRepoMaintenanceFrequency(o.DefaultRepoMaintenanceFrequency),
		WithServiceAccountName(serviceAccountName),
		WithGarbageCollectionFrequency(o.GarbageCollectionFrequency),
		WithUploaderType(o.UploaderType),
	}

	if len(o.Features) > 0 {
		deployOpts = append(deployOpts, WithFeatures(o.Features))
	}

	if o.RestoreOnly {
		deployOpts = append(deployOpts, WithRestoreOnly())
	}

	if len(o.Plugins) > 0 {
		deployOpts = append(deployOpts, WithPlugins(o.Plugins))
	}

	if o.DefaultVolumesToFsBackup {
		deployOpts = append(deployOpts, WithDefaultVolumesToFsBackup())
	}

	deploy := Deployment(o.Namespace, deployOpts...)

	appendUnstructured(resources, deploy)

	if o.UseNodeAgent {
		dsOpts := []podTemplateOption{
			WithAnnotations(o.PodAnnotations),
			WithLabels(o.PodLabels),
			WithImage(o.Image),
			WithResources(o.NodeAgentPodResources),
			WithSecret(secretPresent),
			WithServiceAccountName(serviceAccountName),
		}
		if len(o.Features) > 0 {
			dsOpts = append(dsOpts, WithFeatures(o.Features))
		}
		ds := DaemonSet(o.Namespace, dsOpts...)
		appendUnstructured(resources, ds)
	}

	return resources
}
