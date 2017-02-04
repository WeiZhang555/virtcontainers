//
// Copyright (c) 2016 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package virtcontainers

import (
	"os"
)

// CreatePod is the virtcontainers pod creation entry point.
// CreatePod creates a pod and its containers. It does not start them.
func CreatePod(podConfig PodConfig) (*Pod, error) {
	// Create the pod.
	p, err := createPod(podConfig)
	if err != nil {
		return nil, err
	}

	// Store it.
	err = p.storePod()
	if err != nil {
		return nil, err
	}

	// Initialize the network.
	err = p.network.init(&(p.config.NetworkConfig))
	if err != nil {
		return nil, err
	}

	// Execute prestart hooks inside netns
	err = p.network.run(p.config.NetworkConfig.NetNSPath, func() error {
		return p.config.Hooks.preStartHooks()
	})
	if err != nil {
		return nil, err
	}

	// Add the network
	networkNS, err := p.network.add(*p, p.config.NetworkConfig)
	if err != nil {
		return nil, err
	}

	// Store the network
	err = p.storage.storePodNetwork(p.id, networkNS)
	if err != nil {
		return nil, err
	}

	// Start the VM
	err = p.startVM()
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// DeletePod is the virtcontainers pod deletion entry point.
// DeletePod will stop an already running container and then delete it.
func DeletePod(podID string) (*Pod, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	// Fetch the pod from storage and create it.
	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the network config
	networkNS, err := p.storage.fetchPodNetwork(podID)
	if err != nil {
		return nil, err
	}

	// Stop the VM
	err = p.stopVM()
	if err != nil {
		return nil, err
	}

	// Remove the network
	err = p.network.remove(*p, networkNS)
	if err != nil {
		return nil, err
	}

	// Delete it.
	err = p.delete()
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// StartPod is the virtcontainers pod starting entry point.
// StartPod will talk to the given hypervisor to start an existing
// pod and all its containers.
// It returns the pod ID.
func StartPod(podID string) (*Pod, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	// Fetch the pod from storage and create it.
	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the network config
	networkNS, err := p.storage.fetchPodNetwork(podID)
	if err != nil {
		return nil, err
	}

	// Start it
	err = p.start()
	if err != nil {
		return nil, err
	}

	// Execute poststart hooks inside netns
	err = p.network.run(networkNS.NetNsPath, func() error {
		return p.config.Hooks.postStartHooks()
	})
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// StopPod is the virtcontainers pod stopping entry point.
// StopPod will talk to the given agent to stop an existing pod and destroy all containers within that pod.
func StopPod(podID string) (*Pod, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	// Fetch the pod from storage and create it.
	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the network config
	networkNS, err := p.storage.fetchPodNetwork(podID)
	if err != nil {
		return nil, err
	}

	// Stop it.
	err = p.stop()
	if err != nil {
		p.delete()
		return nil, err
	}

	// Execute poststop hooks inside netns
	err = p.network.run(networkNS.NetNsPath, func() error {
		return p.config.Hooks.postStopHooks()
	})
	if err != nil {
		p.delete()
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// RunPod is the virtcontainers pod running entry point.
// RunPod creates a pod and its containers and then it starts them.
func RunPod(podConfig PodConfig) (*Pod, error) {
	// Create the pod.
	p, err := createPod(podConfig)
	if err != nil {
		return nil, err
	}

	// Store it.
	err = p.storePod()
	if err != nil {
		return nil, err
	}

	lockFile, err := lockPod(p.id)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	// Initialize the network.
	err = p.network.init(&(p.config.NetworkConfig))
	if err != nil {
		return nil, err
	}

	// Execute prestart hooks inside netns
	err = p.network.run(p.config.NetworkConfig.NetNSPath, func() error {
		return p.config.Hooks.preStartHooks()
	})
	if err != nil {
		return nil, err
	}

	// Add the network
	networkNS, err := p.network.add(*p, p.config.NetworkConfig)
	if err != nil {
		return nil, err
	}

	// Store the network
	err = p.storage.storePodNetwork(p.id, networkNS)
	if err != nil {
		return nil, err
	}

	// Start the VM
	err = p.startVM()
	if err != nil {
		return nil, err
	}

	// Start the pod
	err = p.start()
	if err != nil {
		p.delete()
		return nil, err
	}

	// Execute poststart hooks inside netns
	err = p.network.run(networkNS.NetNsPath, func() error {
		return p.config.Hooks.postStartHooks()
	})
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// ListPod is the virtcontainers pod listing entry point.
func ListPod() ([]PodStatus, error) {
	dir, err := os.Open(configStoragePath)
	if err != nil {
		return []PodStatus{}, err
	}

	defer dir.Close()

	pods, err := dir.Readdirnames(0)
	if err != nil {
		return []PodStatus{}, err
	}

	fs := filesystem{}

	var podStatusList []PodStatus

	for _, p := range pods {
		var config PodConfig

		config, err := fs.fetchPodConfig(p)
		if err != nil {
			continue
		}

		state, err := fs.fetchPodState(p)
		if err != nil {
			continue
		}

		podStatus := PodStatus{
			ID:         config.ID,
			State:      state,
			Hypervisor: config.HypervisorType,
			Agent:      config.AgentType,
		}

		podStatusList = append(podStatusList, podStatus)
	}

	return podStatusList, nil
}

// StatusPod is the virtcontainers pod status entry point.
func StatusPod(podID string) (PodStatus, error) {
	fs := filesystem{}

	config, err := fs.fetchPodConfig(podID)
	if err != nil {
		return PodStatus{}, err
	}

	state, err := fs.fetchPodState(podID)
	if err != nil {
		return PodStatus{}, err
	}

	var contStatusList []ContainerStatus
	for _, container := range config.Containers {
		contState, err := fs.fetchContainerState(podID, container.ID)
		if err != nil {
			continue
		}

		contStatus := ContainerStatus{
			ID:    container.ID,
			State: contState,
		}

		contStatusList = append(contStatusList, contStatus)
	}

	podStatus := PodStatus{
		ID:               podID,
		State:            state,
		Hypervisor:       config.HypervisorType,
		Agent:            config.AgentType,
		ContainersStatus: contStatusList,
	}

	return podStatus, nil
}

// CreateContainer is the virtcontainers container creation entry point.
// CreateContainer creates a container on a given pod.
func CreateContainer(podID string, containerConfig ContainerConfig) (*Container, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Create the container.
	c, err := createContainer(p, containerConfig)
	if err != nil {
		return nil, err
	}

	// Store it.
	err = c.storeContainer()
	if err != nil {
		return nil, err
	}

	// Update pod config.
	p.config.Containers = append(p.config.Containers, containerConfig)
	fs := filesystem{}
	err = fs.storePodResource(podID, configFileType, *(p.config))
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// DeleteContainer is the virtcontainers container deletion entry point.
// DeleteContainer deletes a Container from a Pod. If the container is running,
// it needs to be stopped first.
func DeleteContainer(podID, containerID string) (*Container, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the container.
	c, err := fetchContainer(p, containerID)
	if err != nil {
		return nil, err
	}

	// Delete it.
	err = c.delete()
	if err != nil {
		return nil, err
	}

	// Update pod config
	for idx, contConfig := range p.config.Containers {
		if contConfig.ID == containerID {
			p.config.Containers = append(p.config.Containers[:idx], p.config.Containers[idx+1:]...)
			break
		}
	}
	fs := filesystem{}
	err = fs.storePodResource(podID, configFileType, *(p.config))
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// StartContainer is the virtcontainers container starting entry point.
// StartContainer starts an already created container.
func StartContainer(podID, containerID string) (*Container, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the container.
	c, err := fetchContainer(p, containerID)
	if err != nil {
		return nil, err
	}

	// Start it.
	err = c.start()
	if err != nil {
		c.delete()
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// StopContainer is the virtcontainers container stopping entry point.
// StopContainer stops an already running container.
func StopContainer(podID, containerID string) (*Container, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the container.
	c, err := fetchContainer(p, containerID)
	if err != nil {
		return nil, err
	}

	// Stop it.
	err = c.stop()
	if err != nil {
		c.delete()
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// EnterContainer is the virtcontainers container command execution entry point.
// EnterContainer enters an already running container and runs a given command.
func EnterContainer(podID, containerID string, cmd Cmd) (*Container, error) {
	lockFile, err := lockPod(podID)
	if err != nil {
		return nil, err
	}
	defer unlockPod(lockFile)

	p, err := fetchPod(podID)
	if err != nil {
		return nil, err
	}

	// Fetch the container.
	c, err := fetchContainer(p, containerID)
	if err != nil {
		return nil, err
	}

	// Enter it.
	err = c.enter(cmd)
	if err != nil {
		return nil, err
	}

	err = p.endSession()
	if err != nil {
		return nil, err
	}

	return c, nil
}

// StatusContainer is the virtcontainers container status entry point.
// StatusContainer returns a detailed container status.
func StatusContainer(podID, containerID string) (ContainerStatus, error) {
	fs := filesystem{}

	state, err := fs.fetchContainerState(podID, containerID)
	if err != nil {
		return ContainerStatus{}, err
	}

	contStatus := ContainerStatus{
		ID:    containerID,
		State: state,
	}

	return contStatus, nil
}
