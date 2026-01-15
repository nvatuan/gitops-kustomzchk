package security.persistent_volume_forbidden

import rego.v1

# Test case 1: PersistentVolume resource - should deny
test_persistent_volume_denies if {
	test_input := {
		"kind": "PersistentVolume",
		"metadata": {
			"name": "test-pv"
		}
	}
	count(deny) == 1 with input as test_input
}

# Test case 2: PersistentVolumeClaim resource - should deny
test_persistent_volume_claim_denies if {
	test_input := {
		"kind": "PersistentVolumeClaim",
		"metadata": {
			"name": "test-pvc",
			"namespace": "default"
		}
	}
	count(deny) == 1 with input as test_input
}

# Test case 3: Other resource types (Pod) - should be ignored (no denial)
test_pod_resource_ignored if {
	test_input := {
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		}
	}
	count(deny) == 0 with input as test_input
}

# Test case 4: Other resource types (Deployment) - should be ignored (no denial)
test_deployment_resource_ignored if {
	test_input := {
		"kind": "Deployment",
		"metadata": {
			"name": "test-deployment",
			"namespace": "default"
		}
	}
	count(deny) == 0 with input as test_input
}

# Test case 5: Other storage-related resource (ConfigMap) - should be ignored (no denial)
test_configmap_resource_ignored if {
	test_input := {
		"kind": "ConfigMap",
		"metadata": {
			"name": "test-configmap",
			"namespace": "default"
		}
	}
	count(deny) == 0 with input as test_input
}