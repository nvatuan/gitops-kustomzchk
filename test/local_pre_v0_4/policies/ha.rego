package main

import rego.v1

# High Availability Policy
# Ensures deployments have minimum replicas and proper node anti-affinity

# Check if deployment has minimum 2 replicas
deny contains msg if {
    some i
    input[i].contents.kind == "Deployment"
    deployment := input[i].contents
    replicas := deployment.spec.replicas
    replicas < 2
    msg := sprintf("Deployment '%s' must have at least 2 replicas for high availability, found: %d", [deployment.metadata.name, replicas])
}

# Check if deployment has no replicas specified (defaults to 1)
deny contains msg if {
    some i
    input[i].contents.kind == "Deployment"
    deployment := input[i].contents
    not deployment.spec.replicas
    msg := sprintf("Deployment '%s' must have at least 2 replicas for high availability, found: 1 (default)", [deployment.metadata.name])
}

# Check if deployment has PodAntiAffinity OR PodTopologySpread for node distribution
deny contains msg if {
    some i
    input[i].contents.kind == "Deployment"
    deployment := input[i].contents
    not has_ha_distribution(deployment)
    msg := sprintf("Deployment '%s' must have PodAntiAffinity or PodTopologySpread for high availability", [deployment.metadata.name])
}

# Helper function to check if deployment has either PodAntiAffinity or PodTopologySpread
has_ha_distribution(deployment) if {
    has_pod_anti_affinity(deployment)
}

has_ha_distribution(deployment) if {
    has_pod_topology_spread(deployment)
}

# Check if deployment has PodAntiAffinity
has_pod_anti_affinity(deployment) if {
    affinity := deployment.spec.template.spec.affinity
    pod_anti_affinity := affinity.podAntiAffinity
    pod_anti_affinity
}

# Check if deployment has PodTopologySpread
has_pod_topology_spread(deployment) if {
    topology_spread_constraints := deployment.spec.template.spec.topologySpreadConstraints
    topology_spread_constraints
}
