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

package actions

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"k8s.io/kubeadm/kinder/pkg/cluster/status"
	"k8s.io/kubeadm/kinder/pkg/constants"
)

// KubeadmJoin executes the kubeadm join workflow both for control-plane nodes and
// worker nodes
func KubeadmJoin(c *status.Cluster, usePhases, automaticCopyCerts bool, discoveryMode DiscoveryMode, kustomizeDir, patchesDir string, wait time.Duration, vLevel int) (err error) {
	if err := joinControlPlanes(c, usePhases, automaticCopyCerts, discoveryMode, kustomizeDir, patchesDir, wait, vLevel); err != nil {
		return err
	}

	if err := joinWorkers(c, usePhases, discoveryMode, wait, vLevel); err != nil {
		return err
	}
	return nil
}

func joinControlPlanes(c *status.Cluster, usePhases, automaticCopyCerts bool, discoveryMode DiscoveryMode, kustomizeDir, patchesDir string, wait time.Duration, vLevel int) (err error) {
	cpX := []*status.Node{c.BootstrapControlPlane()}

	for _, cp2 := range c.SecondaryControlPlanes().EligibleForActions() {
		// fail fast if required to use kustomize and kubeadm less than v1.17
		if kustomizeDir != "" && cp2.MustKubeadmVersion().LessThan(constants.V1_17) {
			return errors.New("--kustomize-dir can't be used with kubeadm older than v1.17")
		}

		// if kustomize copy patches to the node
		if kustomizeDir != "" {
			if err := copyPatchesToNode(cp2, kustomizeDir); err != nil {
				return err
			}
		}

		// if patcheDir is defined, copy the patches to the node
		if patchesDir != "" {
			if cp2.MustKubeadmVersion().LessThan(constants.V1_19) {
				return errors.New("--patches can't be used with kubeadm older than v1.19")
			}
			if err := copyPatchesToNode(cp2, patchesDir); err != nil {
				return err
			}
		}

		// if not automatic copy certs, simulate manual copy
		if !automaticCopyCerts {
			if err := copyCertificatesToNode(c, cp2); err != nil {
				return err
			}
		}

		// checks pre-loaded images available on the node (this will report missing images, if any)
		kubeVersion, err := cp2.KubeVersion()
		if err != nil {
			return err
		}

		if err := checkImagesForVersion(cp2, kubeVersion); err != nil {
			return err
		}

		// prepares the kubeadm config on this node
		// NB. kubeDNS flag is set to false because it is not relevant for joinConfiguration
		if err := KubeadmJoinConfig(c, automaticCopyCerts, discoveryMode, cp2); err != nil {
			return err
		}

		// executes the kubeadm join control-plane workflow
		if usePhases {
			err = kubeadmJoinControlPlaneWithPhases(cp2, kustomizeDir, patchesDir, vLevel)
		} else {
			err = kubeadmJoinControlPlane(cp2, kustomizeDir, patchesDir, vLevel)
		}
		if err != nil {
			return err
		}

		// updates the loadbalancer config with the new cp node
		cpX = append(cpX, cp2)
		if err := LoadBalancer(c, cpX...); err != nil {
			return err
		}

		if err := waitNewControlPlaneNodeReady(c, cp2, wait); err != nil {
			return err
		}
	}
	return nil
}

func kubeadmJoinControlPlane(cp *status.Node, kustomizeDir, patchesDir string, vLevel int) (err error) {
	joinArgs := []string{
		"join",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
		constants.KubeadmIgnorePreflightErrorsFlag,
	}
	if kustomizeDir != "" {
		joinArgs = append(joinArgs, "-k", constants.PatchesDir)
	}
	if patchesDir != "" {
		joinArgs = append(joinArgs, "--experimental-patches", constants.PatchesDir)
	}

	if err := cp.Command(
		"kubeadm", joinArgs...,
	).RunWithEcho(); err != nil {
		return err
	}

	return nil
}

func kubeadmJoinControlPlaneWithPhases(cp *status.Node, kustomizeDir, patchesDir string, vLevel int) (err error) {
	// kubeadm join phase preflight
	preflightArgs := []string{
		"join", "phase", "preflight",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
		constants.KubeadmIgnorePreflightErrorsFlag,
	}

	if err := cp.Command(
		"kubeadm", preflightArgs...,
	).RunWithEcho(); err != nil {
		return err
	}

	// kubeadm join phase control-plane-prepare
	prepareArgs := []string{
		"join", "phase", "control-plane-prepare", "all",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
	}

	if kustomizeDir != "" {
		prepareArgs = append(prepareArgs, "-k", constants.PatchesDir)
	}
	if patchesDir != "" {
		prepareArgs = append(prepareArgs, "--experimental-patches", constants.PatchesDir)
	}

	if err := cp.Command(
		"kubeadm", prepareArgs...,
	).RunWithEcho(); err != nil {
		return err
	}

	// kubeadm join phase kubelet-start
	if err := cp.Command(
		"kubeadm", "join", "phase", "kubelet-start",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
	).RunWithEcho(); err != nil {
		return err
	}

	// kubeadm join phase control-plane-join
	controlPlaneArgs := []string{
		"join", "phase", "control-plane-join", "all",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
	}
	if kustomizeDir != "" {
		controlPlaneArgs = append(controlPlaneArgs, "-k", constants.PatchesDir)
	}
	if patchesDir != "" {
		controlPlaneArgs = append(controlPlaneArgs, "--experimental-patches", constants.PatchesDir)
	}

	if err := cp.Command(
		"kubeadm", controlPlaneArgs...,
	).RunWithEcho(); err != nil {
		return err
	}

	return nil
}

func joinWorkers(c *status.Cluster, usePhases bool, discoveryMode DiscoveryMode, wait time.Duration, vLevel int) (err error) {
	for _, w := range c.Workers().EligibleForActions() {
		// checks pre-loaded images available on the node (this will report missing images, if any)
		kubeVersion, err := w.KubeVersion()
		if err != nil {
			return err
		}

		if err := checkImagesForVersion(w, kubeVersion); err != nil {
			return err
		}

		// prepares the kubeadm config on this node
		if err := KubeadmJoinConfig(c, false, discoveryMode, w); err != nil {
			return err
		}

		// executes the kubeadm join workflow
		if usePhases {
			err = kubeadmJoinWorkerWithPhases(w, vLevel)
		} else {
			err = kubeadmJoinWorker(w, vLevel)
		}
		if err != nil {
			return err
		}

		if err := waitNewWorkerNodeReady(c, w, wait); err != nil {
			return err
		}
	}
	return nil
}

func kubeadmJoinWorker(w *status.Node, vLevel int) (err error) {
	if err := w.Command(
		"kubeadm", "join",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
		constants.KubeadmIgnorePreflightErrorsFlag,
	).RunWithEcho(); err != nil {
		return err
	}

	return nil
}

func kubeadmJoinWorkerWithPhases(w *status.Node, vLevel int) (err error) {
	// kubeadm join phase preflight
	if err := w.Command(
		"kubeadm", "join", "phase", "preflight",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
		constants.KubeadmIgnorePreflightErrorsFlag,
	).RunWithEcho(); err != nil {
		return err
	}

	// NB. kubeadm join phase control-plane-prepare should not be executed when joining a worker node

	// kubeadm join phase kubelet-start
	if err := w.Command(
		"kubeadm", "join", "phase", "kubelet-start",
		fmt.Sprintf("--config=%s", constants.KubeadmConfigPath),
		fmt.Sprintf("--v=%d", vLevel),
	).RunWithEcho(); err != nil {
		return err
	}

	// NB. kubeadm join phase control-plane-join should not be executed when joining a worker node

	return nil
}
