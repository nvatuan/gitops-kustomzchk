package main

import rego.v1

# Test deployment with valid cpu limit
test_no_cpu_limit_valid_deployment_with_cpu_limit if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
			},
			"spec": {
				"template": {
					"spec": {
						"containers": [{
							"name": "test-container",
							"resources": {
								"requests": {
									"cpu": "100m",
									"memory": "100Mi"
								},
								"limits": {
									"cpu": "1000m",
									"memory": "1000Mi"
								}
							}
						}]
					}
				}
			}
		}
	}]
	count(deny_result) == 1
	"Deployment 'test-deployment' container 'test-container' should not have a cpu limit, found: 1000m" in deny_result
}

# Test deployment without cpu limit
test_no_cpu_limit_valid_deployment_without_cpu_limit if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
			},
			"spec": {
				"template": {
					"spec": {
						"containers": [{
							"name": "test-container",
							"resources": {
								"requests": {
									"cpu": "100m",
									"memory": "100Mi"
								},
								"limits": {
									"memory": "1000Mi"
								}
							}
						}]
					}
				}
			}
		}
	}]
	count(deny_result) == 0
}

# Test deployment without multiple containers, one without cpu limit and one with cpu limit
test_no_cpu_limit_valid_deployment_multiple_containers if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
			},
			"spec": {
				"template": {
					"spec": {
						"containers": [{
							"name": "test-container",
							"resources": {
								"requests": {
									"cpu": "100m",
									"memory": "100Mi"
								},
								"limits": {
									"memory": "1000Mi"
								}
							}
						}, {
							"name": "test-container-2",
							"resources": {
								"requests": {
									"cpu": "100m",
									"memory": "100Mi"
								},
								"limits": {
									"cpu": "500m",
									"memory": "1000Mi"
								}
							}
						}]
					}
				}
			}
		}
	}]
	count(deny_result) == 1
  "Deployment 'test-deployment' container 'test-container-2' should not have a cpu limit, found: 500m" in deny_result
}

# Test deployment with multiple containers, all with cpu limit
test_no_cpu_limit_valid_deployment_multiple_containers if {
	deny_result := data.main.deny with input as [{
		"contents": {
			"kind": "Deployment",
			"metadata": {
				"name": "test-deployment",
			},
			"spec": {
				"template": {
					"spec": {
						"containers": [{
							"name": "test-container",
							"resources": {
								"requests": {
									"cpu": "100m",
									"memory": "100Mi"
								},
								"limits": {
									"cpu": "1000m",
									"memory": "1000Mi"
								}
							}
						}, {
							"name": "test-container-2",
							"resources": {
								"requests": {
									"cpu": "100m",
									"memory": "100Mi"
								},
								"limits": {
									"cpu": "500m",
									"memory": "1000Mi"
								}
							}
						}]
					}
				}
			}
		}
	}]
	count(deny_result) == 2
  "Deployment 'test-deployment' container 'test-container' should not have a cpu limit, found: 1000m" in deny_result
  "Deployment 'test-deployment' container 'test-container-2' should not have a cpu limit, found: 500m" in deny_result
}
