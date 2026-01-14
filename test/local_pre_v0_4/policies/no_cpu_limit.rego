package main

import rego.v1

# No CPU Limit Policy

# Check if deployment has no cpu limit
deny contains msg if {
    some i
    input[i].contents.kind == "Deployment"
    deployment := input[i].contents

    some container in deployment.spec.template.spec.containers
    container.resources.limits.cpu != null
    msg := sprintf("Deployment '%s' container '%s' should not have a cpu limit, found: %s", [
      deployment.metadata.name, container.name, container.resources.limits.cpu
    ])
}
