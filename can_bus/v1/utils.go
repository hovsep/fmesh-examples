package main

func getMCUComponentName(unitName string) string {
	return "mcu-" + unitName
}

func getControllerComponentName(unitName string) string {
	return "controller-" + unitName
}

func getTransceiverComponentName(unitName string) string {
	return "transceiver-" + unitName
}
