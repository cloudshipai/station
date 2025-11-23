package templates

import _ "embed"

//go:embed workflows/build-bundle.yml
var BuildBundleWorkflow string

//go:embed workflows/build-env-image.yml
var BuildEnvImageWorkflow string
