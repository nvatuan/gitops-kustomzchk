# METADATA
# title: Persistent Volume Forbidden
# description: The PersistentVolume and PersistentVolumeClaim resources are forbidden.
# entrypoint: true
# custom:
#  impact: PersistentVolume and PersistentVolumeClaim resources are forbidden. If we allow these, it could lead to data inconsistencies issues and resource management complications in the cluster.
#  category: Deprecation
#  severity: high

package security.persistent_volume_forbidden

import rego.v1

deny contains msg if {
	input.kind == "PersistentVolume"
	msg := sprintf("PersistentVolume %s is forbidden. Please contact the service platform team to discuss alternatives for persistent storage.", [input.metadata.name])
}

deny contains msg if {
	input.kind == "PersistentVolumeClaim"
	msg := sprintf("PersistentVolumeClaim %s in namespace %s is forbidden. Please contact the service platform team to discuss alternatives for persistent storage.", [
		input.metadata.name,
		input.metadata.namespace,
	])
}