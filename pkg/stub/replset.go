package stub

import (
	"fmt"
	"reflect"
	"time"

	"github.com/Percona-Lab/percona-server-mongodb-operator/pkg/apis/psmdb/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	podk8s "github.com/percona/mongodb-orchestration-tools/pkg/pod/k8s"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// handleReplsetInit runs the k8s-mongodb-initiator from within the first running pod's mongod container.
// This must be ran from within the running container to utilise the MongoDB Localhost Exeception.
//
// See: https://docs.mongodb.com/manual/core/security-users/#localhost-exception
//
func (h *Handler) handleReplsetInit(m *v1alpha1.PerconaServerMongoDB, replsetName string, pods []corev1.Pod) error {
	for _, pod := range pods {
		if !isMongodPod(pod) || !isContainerAndPodRunning(pod, mongodContainerName) {
			continue
		}

		logrus.Infof("Initiating replset %s on running pod: %s", replsetName, pod.Name)

		return execCommandInContainer(pod, mongodContainerName, []string{
			"/mongodb/k8s-mongodb-initiator",
			"init",
		})
	}
	return fmt.Errorf("no %s containers in running state", mongodContainerName)
}

func (h *Handler) updateStatus(m *v1alpha1.PerconaServerMongoDB, replsetName string) (*corev1.PodList, error) {
	// Update the PerconaServerMongoDB status with the pod names
	podList := podList()
	labelSelector := labels.SelectorFromSet(labelsForPerconaServerMongoDB(m, replsetName)).String()
	listOps := &metav1.ListOptions{LabelSelector: labelSelector}
	err := h.client.List(m.Namespace, podList, sdk.WithListOptions(listOps))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for replset %s: %v", replsetName, err)
	}
	podNames := getPodNames(podList.Items)

	status := getReplsetStatus(m, replsetName)
	if !reflect.DeepEqual(podNames, status.Members) {
		status.Members = podNames
		err := h.client.Update(m)
		if err != nil {
			return nil, fmt.Errorf("failed to update status for replset %s: %v", replsetName, err)
		}
	}

	// Update the pods list that is read by the watchdog
	if h.pods == nil {
		h.pods = podk8s.NewPods(m.Name, m.Namespace)
	}
	h.pods.SetPods(podList.Items)

	return podList, nil
}

// ensureReplsetStatefulSet ensures a StatefulSet exists
func (h *Handler) ensureReplsetStatefulSet(m *v1alpha1.PerconaServerMongoDB, replsetName string) error {
	set, err := newPSMDBStatefulSet(m, replsetName, nil)
	if err != nil {
		return err
	}
	err = h.client.Create(set)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"size":          m.Spec.Mongod.Size,
			"limit_cpu":     m.Spec.Mongod.Limits.Cpu,
			"limit_memory":  m.Spec.Mongod.Limits.Memory,
			"limit_storage": m.Spec.Mongod.Limits.Storage,
		}).Infof("created stateful set for replset: %s", replsetName)
	}

	// Ensure the stateful set size is the same as the spec
	err = h.client.Get(set)
	if err != nil {
		return fmt.Errorf("failed to get stateful set for replset %s: %v", replsetName, err)
	}
	size := m.Spec.Mongod.Size
	if *set.Spec.Replicas != size {
		logrus.Infof("setting replicas to %d for replset: %s", size, replsetName)
		set.Spec.Replicas = &size
		err = h.client.Update(set)
		if err != nil {
			return fmt.Errorf("failed to update stateful set for replset %s: %v", replsetName, err)
		}
	}

	return nil
}

func getReplsetStatus(m *v1alpha1.PerconaServerMongoDB, replsetName string) *v1alpha1.ReplsetStatus {
	for _, replset := range m.Status.Replsets {
		if replset.Name == replsetName {
			return replset
		}
	}
	replset := &v1alpha1.ReplsetStatus{Name: replsetName}
	m.Status.Replsets = append(m.Status.Replsets, replset)
	return replset
}

func statusHasMember(status *v1alpha1.ReplsetStatus, memberName string) bool {
	for _, member := range status.Members {
		if member == memberName {
			return true
		}
	}
	return false
}

// ensureReplset ensures resources for a PSMDB replset exist
func (h *Handler) ensureReplset(m *v1alpha1.PerconaServerMongoDB, replsetName string) (*v1alpha1.ReplsetStatus, error) {
	// Create the StatefulSet if it doesn't exist
	err := h.ensureReplsetStatefulSet(m, replsetName)
	if err != nil {
		logrus.Errorf("failed to create stateful set for replset %s: %v", replsetName, err)
		return nil, err
	}

	// Update the PSMDB status
	podList, err := h.updateStatus(m, replsetName)
	if err != nil {
		logrus.Errorf("failed to update psmdb status for replset %s: %v", replsetName, err)
		return nil, err
	}

	// Initiate the replset if it hasn't already been initiated + there are pods +
	// we have waited the ReplsetInitWait period since starting
	status := getReplsetStatus(m, replsetName)
	if !status.Initialised && len(podList.Items) >= 1 && time.Since(h.startedAt) > ReplsetInitWait {
		err = h.handleReplsetInit(m, replsetName, podList.Items)
		if err != nil {
			return nil, err
		}

		// update status after replset init
		status.Initialised = true
		err = h.client.Update(m)
		if err != nil {
			return nil, fmt.Errorf("failed to update status for replset %s: %v", replsetName, err)
		}
		logrus.Infof("changed state to initialised for replset %s", replsetName)

		// ensure the watchdog is started
		err = h.ensureWatchdog(m)
		if err != nil {
			return nil, fmt.Errorf("failed to start watchdog: %v", err)
		}
	}

	// Create service for replset
	service := newPSMDBService(m, replsetName)
	err = h.client.Create(service)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			logrus.Errorf("failed to create psmdb service: %v", err)
			return nil, err
		}
	} else {
		logrus.Infof("created service %s", service.Name)
	}

	return status, nil
}
