# METADATA
# title: Pod Minimum Replicas Required
# description: This rule enforces minimum replica count (2) for Deployments to ensure high availability.
# entrypoint: true
# custom:
#  impact: With only 1 replica, your service won't be fault tolerant and may cause downtime because of a single point of failure.
#  category: Reliability
#  severity: high

package reliability.pod_minimum_replicas_required

import rego.v1

# Default to 1 if no replicas specified for Deployment/StatefulSet
get_replica_count(resource) := count if {
	# Check if there's a CKedaScaledObject that overrides this
	ckeda_count := get_ckeda_scaled_object_replica_count(resource.metadata.name, resource.kind)
	ckeda_count > 0
	count := ckeda_count
} else := count if {
	# Use resource spec replicas if > 0
	resource.spec.replicas > 0
	count := resource.spec.replicas
} else := 1

get_ckeda_scaled_object_replica_count(resourceName, resourceType) := count if {
	some i
	input[i].contents.kind == "CKedaScaledObject"
	cKedaScaledObject := input[i].contents
	cKedaScaledObject.spec.scaleTargetRef.name == resourceName
	resourceType in ["StatefulSet", "Deployment"]
	cKedaScaledObject.spec.scaleTargetRef.kind == resourceType
	count := cKedaScaledObject.spec.minReplicaCount
} # Default cKedaScaledObject.spec.scaleTargetRef.kind is Deployment

else := count if {
	some i
	input[i].contents.kind == "CKedaScaledObject"
	cKedaScaledObject := input[i].contents
	cKedaScaledObject.spec.scaleTargetRef.name == resourceName
	resourceType == "Deployment"
	count := cKedaScaledObject.spec.minReplicaCount
} else := 0

# Check if resource has singleton annotation
has_singleton_annotation(resource) if {
	resource.metadata.annotations["service-platform.moneyforward.com/singleton-resource"] == "true"
}

# Check if resource has allow-single-replica annotation
has_allow_single_replica_annotation(resource) if {
	resource.metadata.annotations["service-platform.moneyforward.com/allow-single-replica"] == "true"
}

# Main deny rule for resources with replicas=1 without proper annotations
deny contains msg if {
	# Only apply to relevant resource types
	some i
	input[i].contents.kind in ["Deployment", "StatefulSet"]
	target := input[i].contents

	# Get replica count
	replica_count := get_replica_count(target)

	# Check if replicas is 1
	replica_count == 1

	# Check if missing required annotations
	not has_singleton_annotation(target)
	not has_allow_single_replica_annotation(target)

	msg := sprintf("Resource %s/%s in namespace %s has replicas=1 that is not allowed for High Availability. Please configure more replicas or allow single replica by adding annotation: service-platform.moneyforward.com/allow-single-replica: \"true\". If you want to run as a singleton (one pod across all clusters), add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"", [
		target.kind,
		target.metadata.name,
		target.metadata.namespace,
	])
}

# Deny rule for resources that has singleton annotation but replicas > 1
deny contains msg if {
	# Only apply to relevant resource types
	some i
	input[i].contents.kind in ["Deployment", "StatefulSet"]
	target := input[i].contents

	# Check if resource has singleton annotation
	has_singleton_annotation(target)

	# Get replica count
	replica_count := get_replica_count(target)

	# Check if replicas is greater than 1
	replica_count > 1

	msg := sprintf("Resource %s in namespace %s has singleton annotation but replicas=%d. Remove annotation: service-platform.moneyforward.com/singleton-resource or set replicas to 1", [
		target.metadata.name,
		target.metadata.namespace,
		replica_count,
	])
}

# Deny for resources (CronJob, Job, KafkaConnect, KafkaConnector) that are required to run in one cluster but has no singleton annotation
deny contains msg if {
	some i
	input[i].contents.kind in ["CronJob", "Job", "KafkaConnect", "KafkaConnector"]
	target := input[i].contents

	not has_singleton_annotation(target)

	msg := sprintf("Resource %s in namespace %s has no singleton annotation. Add annotation: service-platform.moneyforward.com/singleton-resource: \"true\"", [
		target.metadata.name,
		target.metadata.namespace,
	])
}