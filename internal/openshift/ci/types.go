package ci

// Source: https://github.com/openshift/release/blob/master/core-services/prow/02_config/_config.yaml#L84
var CommonErrorPatterns = []string{
	`error:`,
	`Failed to push image`,
	`Failed`,
	`timed out`,
	`'ERROR:'`,
	`ERRO\[`,
	`^error:`,
	`(^FAIL|FAIL: |Failure \[)\b`,
	`panic(\.go)?:`,
	`"level":"error"`,
	`level=error`,
	`level":"fatal"`,
	`level=fatal`,
	`│ Error:`,
	`client connection lost`,
}
