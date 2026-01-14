package reliability.pod_minimum_replicas_required

import rego.v1

# Test data for valid Deployment with 2 replicas
valid_deployment_2_replicas := [{"contents": {
	"kind": "Deployment",
	"metadata": {
		"name": "valid-deployment",
		"namespace": "test-namespace",
	},
	"spec": {
		"replicas": 2,
		"selector": {"matchLabels": {"app": "test-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "nginx:latest",
			}]},
		},
	},
}}]

# Test data for Deployment with 1 replica but no annotations (should be denied)
deployment_1_replica_no_annotations := [{"contents": {
	"kind": "Deployment",
	"metadata": {
		"name": "single-replica-deployment",
		"namespace": "test-namespace",
	},
	"spec": {
		"replicas": 1,
		"selector": {"matchLabels": {"app": "test-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "nginx:latest",
			}]},
		},
	},
}}]

# Test data for Deployment with 1 replica and singleton annotation (should pass)
deployment_1_replica_with_singleton := [{"contents": {
	"kind": "Deployment",
	"metadata": {
		"name": "singleton-deployment",
		"namespace": "test-namespace",
		"annotations": {"service-platform.moneyforward.com/singleton-resource": "true"},
	},
	"spec": {
		"replicas": 1,
		"selector": {"matchLabels": {"app": "test-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "nginx:latest",
			}]},
		},
	},
}}]

# Test data for Deployment with 1 replica and allow-single-replica annotation (should pass)
deployment_1_replica_with_allow_single := [{"contents": {
	"kind": "Deployment",
	"metadata": {
		"name": "allow-single-deployment",
		"namespace": "test-namespace",
		"annotations": {"service-platform.moneyforward.com/allow-single-replica": "true"},
	},
	"spec": {
		"replicas": 1,
		"selector": {"matchLabels": {"app": "test-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "nginx:latest",
			}]},
		},
	},
}}]

# Test data for Deployment with singleton annotation but 3 replicas (should be denied)
deployment_singleton_with_multiple_replicas := [{"contents": {
	"kind": "Deployment",
	"metadata": {
		"name": "singleton-multi-replica-deployment",
		"namespace": "test-namespace",
		"annotations": {"service-platform.moneyforward.com/singleton-resource": "true"},
	},
	"spec": {
		"replicas": 3,
		"selector": {"matchLabels": {"app": "test-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "nginx:latest",
			}]},
		},
	},
}}]

# Test data for StatefulSet with 1 replica but no annotations (should be denied)
statefulset_1_replica_no_annotations := [{"contents": {
	"kind": "StatefulSet",
	"metadata": {
		"name": "single-replica-statefulset",
		"namespace": "test-namespace",
	},
	"spec": {
		"serviceName": "test-service",
		"replicas": 1,
		"selector": {"matchLabels": {"app": "test-stateful-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-stateful-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "postgres:13",
			}]},
		},
	},
}}]

# Test data for CronJob without singleton annotation (should be denied)
cronjob_no_singleton := [{"contents": {
	"kind": "CronJob",
	"metadata": {
		"name": "test-cronjob",
		"namespace": "test-namespace",
	},
	"spec": {
		"schedule": "0 2 * * *",
		"jobTemplate": {"spec": {"template": {"spec": {
			"restartPolicy": "OnFailure",
			"containers": [{
				"name": "cronjob-container",
				"image": "busybox:latest",
				"command": ["echo", "Hello"],
			}],
		}}}},
	},
}}]

# Test data for CronJob with singleton annotation (should pass)
cronjob_with_singleton := [{"contents": {
	"kind": "CronJob",
	"metadata": {
		"name": "singleton-cronjob",
		"namespace": "test-namespace",
		"annotations": {"service-platform.moneyforward.com/singleton-resource": "true"},
	},
	"spec": {
		"schedule": "0 2 * * *",
		"jobTemplate": {"spec": {"template": {"spec": {
			"restartPolicy": "OnFailure",
			"containers": [{
				"name": "cronjob-container",
				"image": "busybox:latest",
				"command": ["echo", "Hello"],
			}],
		}}}},
	},
}}]

# Test data for Job without singleton annotation (should be denied)
job_no_singleton := [{"contents": {
	"kind": "Job",
	"metadata": {
		"name": "test-job",
		"namespace": "test-namespace",
	},
	"spec": {"template": {"spec": {
		"restartPolicy": "Never",
		"containers": [{
			"name": "job-container",
			"image": "busybox:latest",
			"command": ["echo", "Hello World"],
		}],
	}}},
}}]

# Test data for KafkaConnect without singleton annotation (should be denied)
kafkaconnect_no_singleton := [{"contents": {
	"kind": "KafkaConnect",
	"metadata": {
		"name": "test-kafkaconnect",
		"namespace": "test-namespace",
	},
	"spec": {
		"replicas": 1,
		"bootstrapServers": "kafka:9092",
	},
}}]

# Test data for KafkaConnector without singleton annotation (should be denied)
kafkaconnector_no_singleton := [{"contents": {
	"kind": "KafkaConnector",
	"metadata": {
		"name": "test-kafkaconnector",
		"namespace": "test-namespace",
	},
	"spec": {
		"class": "org.apache.kafka.connect.file.FileStreamSourceConnector",
		"tasksMax": 1,
	},
}}]

# Test data for Deployment with CKedaScaledObject override
deployment_with_ckeda_override := [
	{"contents": {
		"kind": "Deployment",
		"metadata": {
			"name": "ckeda-deployment",
			"namespace": "test-namespace",
		},
		"spec": {
			"replicas": 1,
			"selector": {"matchLabels": {"app": "ckeda-app"}},
			"template": {
				"metadata": {"labels": {"app": "ckeda-app"}},
				"spec": {"containers": [{
					"name": "test-container",
					"image": "nginx:latest",
				}]},
			},
		},
	}},
	{"contents": {
		"kind": "CKedaScaledObject",
		"metadata": {
			"name": "ckeda-scaler",
			"namespace": "test-namespace",
		},
		"spec": {
			"scaleTargetRef": {
				"name": "ckeda-deployment",
				"kind": "Deployment",
			},
			"minReplicaCount": 2,
			"maxReplicaCount": 10,
		},
	}},
]

# Test data for Deployment with CKedaScaledObject but minReplicaCount=1 (should be denied)
deployment_with_ckeda_min_1 := [
	{"contents": {
		"kind": "Deployment",
		"metadata": {
			"name": "ckeda-min-1-deployment",
			"namespace": "test-namespace",
		},
		"spec": {
			"replicas": 1,
			"selector": {"matchLabels": {"app": "ckeda-app"}},
			"template": {
				"metadata": {"labels": {"app": "ckeda-app"}},
				"spec": {"containers": [{
					"name": "test-container",
					"image": "nginx:latest",
				}]},
			},
		},
	}},
	{"contents": {
		"kind": "CKedaScaledObject",
		"metadata": {
			"name": "ckeda-scaler-min-1",
			"namespace": "test-namespace",
		},
		"spec": {
			"scaleTargetRef": {
				"name": "ckeda-min-1-deployment",
				"kind": "Deployment",
			},
			"minReplicaCount": 1,
			"maxReplicaCount": 5,
		},
	}},
]

# Test data for StatefulSet with CKedaScaledObject
statefulset_with_ckeda := [
	{"contents": {
		"kind": "StatefulSet",
		"metadata": {
			"name": "ckeda-statefulset",
			"namespace": "test-namespace",
		},
		"spec": {
			"serviceName": "test-service",
			"replicas": 1,
			"selector": {"matchLabels": {"app": "ckeda-stateful-app"}},
			"template": {
				"metadata": {"labels": {"app": "ckeda-stateful-app"}},
				"spec": {"containers": [{
					"name": "test-container",
					"image": "postgres:13",
				}]},
			},
		},
	}},
	{"contents": {
		"kind": "CKedaScaledObject",
		"metadata": {
			"name": "ckeda-stateful-scaler",
			"namespace": "test-namespace",
		},
		"spec": {
			"scaleTargetRef": {
				"name": "ckeda-statefulset",
				"kind": "StatefulSet",
			},
			"minReplicaCount": 3,
			"maxReplicaCount": 8,
		},
	}},
]

# Test data for Deployment with no replicas specified (defaults to 1)
deployment_no_replicas_specified := [{"contents": {
	"kind": "Deployment",
	"metadata": {
		"name": "no-replicas-deployment",
		"namespace": "test-namespace",
	},
	"spec": {
		"selector": {"matchLabels": {"app": "test-app"}},
		"template": {
			"metadata": {"labels": {"app": "test-app"}},
			"spec": {"containers": [{
				"name": "test-container",
				"image": "nginx:latest",
			}]},
		},
	},
}}]

# Test data for non-covered resource types (should be ignored)
service_resource := [{"contents": {
	"kind": "Service",
	"metadata": {
		"name": "test-service",
		"namespace": "test-namespace",
	},
	"spec": {
		"selector": {"app": "test-app"},
		"ports": [{
			"port": 80,
			"targetPort": 8080,
		}],
	},
}}]

# Test data for mixed resources
mixed_resources := [
	{"contents": {
		"kind": "Deployment",
		"metadata": {
			"name": "valid-deployment",
			"namespace": "test-namespace",
		},
		"spec": {
			"replicas": 2,
			"selector": {"matchLabels": {"app": "test-app"}},
			"template": {
				"metadata": {"labels": {"app": "test-app"}},
				"spec": {"containers": [{
					"name": "test-container",
					"image": "nginx:latest",
				}]},
			},
		},
	}},
	{"contents": {
		"kind": "CronJob",
		"metadata": {
			"name": "invalid-cronjob",
			"namespace": "test-namespace",
		},
		"spec": {
			"schedule": "0 2 * * *",
			"jobTemplate": {"spec": {"template": {"spec": {
				"restartPolicy": "OnFailure",
				"containers": [{
					"name": "cronjob-container",
					"image": "busybox:latest",
				}],
			}}}},
		},
	}},
	{"contents": {
		"kind": "Service",
		"metadata": {
			"name": "test-service",
			"namespace": "test-namespace",
		},
		"spec": {
			"selector": {"app": "test-app"},
			"ports": [{
				"port": 80,
				"targetPort": 8080,
			}],
		},
	}},
]

# Tests for valid resources (should pass)
test_valid_deployment_2_replicas_passes if {
	count(deny) == 0 with input as valid_deployment_2_replicas
}

test_deployment_1_replica_with_singleton_passes if {
	count(deny) == 0 with input as deployment_1_replica_with_singleton
}

test_deployment_1_replica_with_allow_single_passes if {
	count(deny) == 0 with input as deployment_1_replica_with_allow_single
}

test_cronjob_with_singleton_passes if {
	count(deny) == 0 with input as cronjob_with_singleton
}

test_deployment_with_ckeda_override_passes if {
	count(deny) == 0 with input as deployment_with_ckeda_override
}

test_statefulset_with_ckeda_passes if {
	count(deny) == 0 with input as statefulset_with_ckeda
}

# Tests for invalid resources (should be denied)
test_deny_deployment_1_replica_no_annotations if {
	count(deny) == 1 with input as deployment_1_replica_no_annotations
	some msg in deny with input as deployment_1_replica_no_annotations
	contains(msg, "Resource Deployment/single-replica-deployment in namespace test-namespace has replicas=1")
}

test_deny_deployment_singleton_with_multiple_replicas if {
	count(deny) == 1 with input as deployment_singleton_with_multiple_replicas
	some msg in deny with input as deployment_singleton_with_multiple_replicas
	contains(msg, "Resource singleton-multi-replica-deployment in namespace test-namespace has singleton annotation but replicas=3")
	contains(msg, "Remove annotation: service-platform.moneyforward.com/singleton-resource or set replicas to 1")
}

test_deny_statefulset_1_replica_no_annotations if {
	count(deny) == 1 with input as statefulset_1_replica_no_annotations
	some msg in deny with input as statefulset_1_replica_no_annotations
	contains(msg, "Resource StatefulSet/single-replica-statefulset in namespace test-namespace has replicas=1")
}

test_deny_cronjob_no_singleton if {
	count(deny) == 1 with input as cronjob_no_singleton
	some msg in deny with input as cronjob_no_singleton
	contains(msg, "Resource test-cronjob in namespace test-namespace has no singleton annotation")
	contains(msg, "Add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"")
}

test_deny_job_no_singleton if {
	count(deny) == 1 with input as job_no_singleton
	some msg in deny with input as job_no_singleton
	contains(msg, "Resource test-job in namespace test-namespace has no singleton annotation")
	contains(msg, "Add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"")
}

test_deny_kafkaconnect_no_singleton if {
	count(deny) == 1 with input as kafkaconnect_no_singleton
	some msg in deny with input as kafkaconnect_no_singleton
	contains(msg, "Resource test-kafkaconnect in namespace test-namespace has no singleton annotation")
	contains(msg, "Add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"")
}

test_deny_kafkaconnector_no_singleton if {
	count(deny) == 1 with input as kafkaconnector_no_singleton
	some msg in deny with input as kafkaconnector_no_singleton
	contains(msg, "Resource test-kafkaconnector in namespace test-namespace has no singleton annotation")
	contains(msg, "Add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"")
}

test_deny_deployment_with_ckeda_min_1 if {
	count(deny) == 1 with input as deployment_with_ckeda_min_1
	some msg in deny with input as deployment_with_ckeda_min_1
	contains(msg, "Resource Deployment/ckeda-min-1-deployment in namespace test-namespace has replicas=1")
}

test_deny_deployment_no_replicas_specified if {
	count(deny) == 1 with input as deployment_no_replicas_specified
	some msg in deny with input as deployment_no_replicas_specified
	contains(msg, "Resource Deployment/no-replicas-deployment in namespace test-namespace has replicas=1")
}

# Tests for non-covered resource types (should be ignored)
test_service_resource_ignored if {
	count(deny) == 0 with input as service_resource
}

# Tests for mixed resources
test_mixed_resources if {
	count(deny) == 1 with input as mixed_resources
	some msg in deny with input as mixed_resources
	contains(msg, "Resource invalid-cronjob in namespace test-namespace has no singleton annotation")
	contains(msg, "Add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"")
}

# Edge case: Test CKedaScaledObject without matching deployment
test_ckeda_without_matching_deployment if {
	orphaned_ckeda := [{"contents": {
		"kind": "CKedaScaledObject",
		"metadata": {
			"name": "orphaned-scaler",
			"namespace": "test-namespace",
		},
		"spec": {
			"scaleTargetRef": {
				"name": "non-existent-deployment",
				"kind": "Deployment",
			},
			"minReplicaCount": 2,
			"maxReplicaCount": 10,
		},
	}}]

	# Should pass because there's no deployment to validate
	count(deny) == 0 with input as orphaned_ckeda
}

# Edge case: Test deployment with both annotations
test_deployment_with_both_annotations if {
	deployment_both_annotations := [{"contents": {
		"kind": "Deployment",
		"metadata": {
			"name": "both-annotations-deployment",
			"namespace": "test-namespace",
			"annotations": {
				"service-platform.moneyforward.com/singleton-resource": "true",
				"service-platform.moneyforward.com/allow-single-replica": "true",
			},
		},
		"spec": {
			"replicas": 1,
			"selector": {"matchLabels": {"app": "test-app"}},
			"template": {
				"metadata": {"labels": {"app": "test-app"}},
				"spec": {"containers": [{
					"name": "test-container",
					"image": "nginx:latest",
				}]},
			},
		},
	}}]

	# Should pass because it has at least one valid annotation
	count(deny) == 0 with input as deployment_both_annotations
}

# Edge case: Test CKedaScaledObject with default kind (Deployment)
test_ckeda_default_kind if {
	ckeda_default_kind := [
		{"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "default-kind-deployment",
				"namespace": "test-namespace",
			},
			"spec": {
				"replicas": 1,
				"selector": {"matchLabels": {"app": "test-app"}},
				"template": {
					"metadata": {"labels": {"app": "test-app"}},
					"spec": {"containers": [{
						"name": "test-container",
						"image": "nginx:latest",
					}]},
				},
			},
		}},
		{"contents": {
			"kind": "CKedaScaledObject",
			"metadata": {
				"name": "default-kind-scaler",
				"namespace": "test-namespace",
			},
			"spec": {
				"scaleTargetRef": {"name": "default-kind-deployment"},
				"minReplicaCount": 2,
				"maxReplicaCount": 10,
			},
		}},
	]

	# Should pass because CKedaScaledObject defaults to Deployment kind and sets minReplicaCount=2
	count(deny) == 0 with input as ckeda_default_kind
}

# Edge case: Test deployment with replicas=0
test_deployment_zero_replicas if {
	deployment_zero_replicas := [{"contents": {
		"kind": "Deployment",
		"metadata": {
			"name": "zero-replicas-deployment",
			"namespace": "test-namespace",
		},
		"spec": {
			"replicas": 0,
			"selector": {"matchLabels": {"app": "test-app"}},
			"template": {
				"metadata": {"labels": {"app": "test-app"}},
				"spec": {"containers": [{
					"name": "test-container",
					"image": "nginx:latest",
				}]},
			},
		},
	}}]

	# Should be denied because replicas=0 defaults to 1 in the policy logic
	count(deny) == 1 with input as deployment_zero_replicas
	some msg in deny with input as deployment_zero_replicas
	contains(msg, "Resource Deployment/zero-replicas-deployment in namespace test-namespace has replicas=1")
}