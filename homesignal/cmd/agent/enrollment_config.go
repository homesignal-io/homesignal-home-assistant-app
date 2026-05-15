package main

import (
	"os"
	"strings"
)

func loadEnrollmentConfig(options OptionsState) EnrollmentConfig {
	config := EnrollmentConfig{
		HomeSignalAPIBaseURL:          defaultHomeSignalAPIBaseURL,
		AWSRegion:                     defaultAWSRegion,
		FleetProvisioningTemplateName: defaultFleetProvisioningTemplateName,
		HomeSignalAPISource:           "default",
		AWSIoTEndpointSource:          "unset",
	}

	config.HomeSignalAPIBaseURL, config.HomeSignalAPISource = optionString(options, "cloud_base_url", config.HomeSignalAPIBaseURL, config.HomeSignalAPISource)
	config.AWSIoTEndpoint, config.AWSIoTEndpointSource = optionString(options, "aws_iot_endpoint", config.AWSIoTEndpoint, config.AWSIoTEndpointSource)
	config.AWSRegion, _ = optionString(options, "aws_region", config.AWSRegion, "default")
	config.FleetProvisioningTemplateName, _ = optionString(options, "fleet_provisioning_template", config.FleetProvisioningTemplateName, "default")

	config.HomeSignalAPIBaseURL, config.HomeSignalAPISource = envString("HOMESIGNAL_API_BASE_URL", config.HomeSignalAPIBaseURL, config.HomeSignalAPISource)
	config.AWSIoTEndpoint, config.AWSIoTEndpointSource = envString("AWS_IOT_ENDPOINT", config.AWSIoTEndpoint, config.AWSIoTEndpointSource)
	config.AWSRegion, _ = envString("AWS_REGION", config.AWSRegion, "default")
	config.FleetProvisioningTemplateName, _ = envString("AWS_IOT_FLEET_PROVISIONING_TEMPLATE", config.FleetProvisioningTemplateName, "default")

	return config
}

func optionString(options OptionsState, key, fallback, source string) (string, string) {
	value, ok := options.Options[key]
	if !ok {
		return fallback, source
	}
	text, ok := value.(string)
	if !ok {
		return fallback, source
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fallback, source
	}
	return text, "option"
}

func envString(key, fallback, source string) (string, string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, source
	}
	return value, "env"
}
