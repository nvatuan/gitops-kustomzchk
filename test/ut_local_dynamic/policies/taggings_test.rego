package main

import rego.v1

budget_component_label := "github.com/nvatuan/domains"

# Test Deployment with valid budget component label
test_deployment_with_valid_label if {
	deny_result := data.main.deny with input as {
		"kind": "Deployment",
		"metadata": {"name": "test-deployment"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"github.com/nvatuan/domains": "my-service"
					}
				}
			}
		}
	}
	count(deny_result) == 0
}

# Test Deployment without budget component label
test_deployment_without_label if {
	deny_result := data.main.deny with input as {
		"kind": "Deployment",
		"metadata": {"name": "test-deployment"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {}
				}
			}
		}
	}
	deny_result == {"Deployment test-deployment does not have the required label 'github.com/nvatuan/domains'"}
}

# Test StatefulSet with valid budget component label
test_statefulset_with_valid_label if {
	deny_result := data.main.deny with input as {
		"kind": "StatefulSet",
		"metadata": {"name": "test-statefulset"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"github.com/nvatuan/domains": "my-service"
					}
				}
			}
		}
	}
	count(deny_result) == 0
}

# Test StatefulSet without budget component label
test_statefulset_without_label if {
	deny_result := data.main.deny with input as {
		"kind": "StatefulSet",
		"metadata": {"name": "test-statefulset"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {}
				}
			}
		}
	}
	deny_result == {"StatefulSet test-statefulset does not have the required label 'github.com/nvatuan/domains'"}
}

# Test Job with valid budget component label
test_job_with_valid_label if {
	deny_result := data.main.deny with input as {
		"kind": "Job",
		"metadata": {"name": "test-job"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"github.com/nvatuan/domains": "my-service"
					}
				}
			}
		}
	}
	count(deny_result) == 0
}

# Test Job without budget component label
test_job_without_label if {
	deny_result := data.main.deny with input as {
		"kind": "Job",
		"metadata": {"name": "test-job"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {}
				}
			}
		}
	}
	deny_result == {"Job test-job does not have the required label 'github.com/nvatuan/domains'"}
}

# Test CronJob with valid budget component label
test_cronjob_with_valid_label if {
	deny_result := data.main.deny with input as {
		"kind": "CronJob",
		"metadata": {"name": "test-cronjob"},
		"spec": {
			"jobTemplate": {
				"spec": {
					"template": {
						"metadata": {
							"labels": {
								"github.com/nvatuan/domains": "my-service"
							}
						}
					}
				}
			}
		}
	}
	count(deny_result) == 0
}

# Test CronJob without budget component label
test_cronjob_without_label if {
	deny_result := data.main.deny with input as {
		"kind": "CronJob",
		"metadata": {"name": "test-cronjob"},
		"spec": {
			"jobTemplate": {
				"spec": {
					"template": {
						"metadata": {
							"labels": {}
						}
					}
				}
			}
		}
	}
	deny_result == {"CronJob test-cronjob does not have the required label 'github.com/nvatuan/domains'"}
}

# Test other Kubernetes kinds (should pass)
test_other_kinds if {
	deny_result := data.main.deny with input as {
		"kind": "Service",
		"metadata": {"name": "test-service"},
		"spec": {}
	}
	count(deny_result) == 0
}

test_configmap if {
	deny_result := data.main.deny with input as {
		"kind": "ConfigMap",
		"metadata": {"name": "test-configmap"},
		"data": {}
	}
	count(deny_result) == 0
}

test_secret if {
	deny_result := data.main.deny with input as {
		"kind": "Secret",
		"metadata": {"name": "test-secret"},
		"type": "Opaque"
	}
	count(deny_result) == 0
}

# Test edge cases
test_deployment_with_empty_labels if {
	deny_result := data.main.deny with input as {
		"kind": "Deployment",
		"metadata": {"name": "test-deployment"},
		"spec": {
			"template": {
				"metadata": {
					"labels": null
				}
			}
		}
	}
	deny_result == {"Deployment test-deployment does not have the required label 'github.com/nvatuan/domains'"}
}

test_deployment_with_other_labels if {
	deny_result := data.main.deny with input as {
		"kind": "Deployment",
		"metadata": {"name": "test-deployment"},
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"app": "my-app",
						"version": "v1.0.0"
					}
				}
			}
		}
	}
	deny_result == {"Deployment test-deployment does not have the required label 'github.com/nvatuan/domains'"}
}

# Test budget_component_label constant
test_budget_component_label_constant if {
	budget_component_label == "github.com/nvatuan/domains"
}

