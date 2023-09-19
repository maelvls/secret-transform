{{- define "secret-transform.name" -}}
{{- $.Values.nameOverride | default $.Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "secret-transform.selectorLabels" -}}
{{- tpl (toYaml $.Values.selectorLabels) $ }}
{{- end }}
