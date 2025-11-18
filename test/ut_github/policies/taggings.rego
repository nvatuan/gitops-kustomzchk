package main

import rego.v1

budget_component_label := "github.com/nvatuan/domains"

deny contains msg if {
  some i
  input[i].contents.kind == "Deployment"
  deployment := input[i].contents
  not deployment.spec.template.metadata.labels[budget_component_label]
  msg = sprintf("Deployment %s does not have the required label '%s'", [deployment.metadata.name, budget_component_label])
}

deny contains msg if {
  some i
  input[i].contents.kind == "StatefulSet"
  statefulSet := input[i].contents
  not statefulSet.spec.template.metadata.labels[budget_component_label]
  msg = sprintf("StatefulSet %s does not have the required label '%s'", [statefulSet.metadata.name, budget_component_label])
}

deny contains msg if {
  some i
  input[i].contents.kind == "Job"
  job := input[i].contents
  not job.spec.template.metadata.labels[budget_component_label]
  msg = sprintf("Job %s does not have the required label '%s'", [job.metadata.name, budget_component_label])
}

deny contains msg if {
  some i
  input[i].contents.kind == "CronJob"
  cronJob := input[i].contents
  not cronJob.spec.jobTemplate.spec.template.metadata.labels[budget_component_label]
  msg = sprintf("CronJob %s does not have the required label '%s'", [cronJob.metadata.name, budget_component_label])
}