package main

import rego.v1

# Test deployment with valid HA configuration (2+ replicas, PodAntiAffinity)
test_ha_valid_deployment_with_anti_affinity if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
				"labels": {"app": "test-app"}
			},
			"spec": {
				"replicas": 3,
				"template": {
					"spec": {
						"affinity": {
							"podAntiAffinity": {
								"requiredDuringSchedulingIgnoredDuringExecution": [{
									"topologyKey": "kubernetes.io/hostname",
									"labelSelector": {
										"matchExpressions": [{
											"key": "app",
											"operator": "In",
											"values": ["test-app"]
										}]
									}
								}]
							}
						}
					}
				}
			}
		}
	}]
	count(deny_result) == 0
}

# Test deployment with valid HA configuration (2+ replicas, PodTopologySpread)
test_ha_valid_deployment_with_topology_spread if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
				"labels": {"app": "test-app"}
			},
			"spec": {
				"replicas": 3,
				"template": {
					"spec": {
						"topologySpreadConstraints": [{
							"maxSkew": 1,
							"topologyKey": "kubernetes.io/hostname",
							"whenUnsatisfiable": "DoNotSchedule",
							"labelSelector": {
								"matchLabels": {"app": "test-app"}
							}
						}]
					}
				}
			}
		}
	}]
	count(deny_result) == 0
}

# Test deployment with insufficient replicas
test_ha_insufficient_replicas if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {"name": "test-deployment"},
			"spec": {"replicas": 1}
		}
	}]
	count(deny_result) == 2
	"Deployment 'test-deployment' must have at least 2 replicas for high availability, found: 1" in deny_result
	"Deployment 'test-deployment' must have PodAntiAffinity or PodTopologySpread for high availability" in deny_result
}

# Test deployment with no replicas specified (defaults to 1)
test_ha_no_replicas_specified if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {"name": "test-deployment"},
			"spec": {}
		}
	}]
	count(deny_result) == 2
	"Deployment 'test-deployment' must have at least 2 replicas for high availability, found: 1 (default)" in deny_result
	"Deployment 'test-deployment' must have PodAntiAffinity or PodTopologySpread for high availability" in deny_result
}

# Test deployment with no HA distribution (no PodAntiAffinity or PodTopologySpread)
test_ha_no_ha_distribution if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
				"labels": {"app": "test-app"}
			},
			"spec": {
				"replicas": 2,
				"template": {
					"spec": {}
				}
			}
		}
	}]
	count(deny_result) == 1
	"Deployment 'test-deployment' must have PodAntiAffinity or PodTopologySpread for high availability" in deny_result
}

# Test deployment with both issues (insufficient replicas and no HA distribution)
test_ha_both_issues if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {"name": "test-deployment"},
			"spec": {"replicas": 1}
		}
	}]
	count(deny_result) == 2
	"Deployment 'test-deployment' must have at least 2 replicas for high availability, found: 1" in deny_result
	"Deployment 'test-deployment' must have PodAntiAffinity or PodTopologySpread for high availability" in deny_result
}

# Test non-deployment resource (should pass)
test_ha_non_deployment if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Service",
			"metadata": {"name": "test-service"},
			"spec": {}
		}
	}]
	count(deny_result) == 0
}

# Test multiple resources (Service + Deployment)
test_ha_multiple_resources if {
	deny_result := data.main.deny with input as [
		{
			"contents": {
				"kind": "Service",
				"metadata": {"name": "test-service"},
				"spec": {}
			}
		},
		{
			"contents": {
				"kind": "Deployment",
				"metadata": {"name": "test-deployment"},
				"spec": {"replicas": 1}
			}
		}
	]
	count(deny_result) == 2
	"Deployment 'test-deployment' must have at least 2 replicas for high availability, found: 1" in deny_result
	"Deployment 'test-deployment' must have PodAntiAffinity or PodTopologySpread for high availability" in deny_result
}