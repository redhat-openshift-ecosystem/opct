/*
Handle items in the file path meta/run.log
*/
package archive

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type MetaLogItem struct {
	Level      string `json:"level,omitempty"`
	Message    string `json:"msg,omitempty"`
	Time       string `json:"time,omitempty"`
	Plugin     string `json:"plugin,omitempty"`
	Method     string `json:"method,omitempty"`
	PluginName string `json:"plugin_name,omitempty"`
}

func ParseMetaLogs(logs []string) []*RuntimeInfoItem {

	var serverStartedAt string
	runtimeLogs := []*RuntimeInfoItem{}
	exists := struct{}{}
	mapExists := map[string]struct{}{}
	pluginStartedAt := map[string]string{}
	pluginFinishedAt := map[string]string{}

	// convert from ISO8601 [returning errors] to:
	dateFormat := "2006/01/02 15:04:05"
	convertDate := func(t string) string {
		new := strings.Replace(t, "-", "/", -1)
		new = strings.Replace(new, "T", " ", -1)
		new = strings.Replace(new, "Z", "", -1)
		return new
	}
	diffDate := func(strStart string, strEnd string) string {
		start, err := time.Parse(dateFormat, convertDate(strStart))
		if err != nil {
			fmt.Println("start")
			fmt.Println(err)
		}
		end, err := time.Parse(dateFormat, convertDate(strEnd))
		if err != nil {
			fmt.Println("dateEnd")
			fmt.Println(err)
		}
		return end.Sub(start).String()
	}

	// parse meta/run.log
	for x := range logs {
		logitem := MetaLogItem{}
		if err := json.Unmarshal([]byte(logs[x]), &logitem); err != nil {
			log.Debugf("Erorr: [parser] couldn't parse item in meta/run.log: %v", err)
			continue
		}

		// server started: msg=Starting server Expected Results
		if strings.HasPrefix(logitem.Message, "Starting server Expected Results") {
			runtimeLogs = append(runtimeLogs, &RuntimeInfoItem{
				Time: logitem.Time,
				Name: "server started",
			})
			serverStartedAt = logitem.Time
		}

		// marker: plugin started (healthy)
		if logitem.Method == "POST" && logitem.Message == "received request" {
			// Get only the first message indicating the plugin has been started
			if _, ok := mapExists[logitem.PluginName]; ok {
				continue
			}
			mapExists[logitem.PluginName] = exists
			runtimeLogs = append(runtimeLogs, &RuntimeInfoItem{
				Time: logitem.Time,
				Name: fmt.Sprintf("plugin started %s", logitem.PluginName),
			})
			pluginStartedAt[logitem.PluginName] = logitem.Time
		}

		// marker: plugin finished
		if logitem.Method == "PUT" {
			pluginFinishedAt[logitem.PluginName] = logitem.Time
			var delta string
			switch logitem.PluginName {
			case "05-openshift-cluster-upgrade":
				delta = diffDate(pluginStartedAt[logitem.PluginName], logitem.Time)
			case "10-openshift-kube-conformance":
				delta = diffDate(pluginFinishedAt["05-openshift-cluster-upgrade"], logitem.Time)
			case "20-openshift-conformance-validated":
				delta = diffDate(pluginFinishedAt["10-openshift-kube-conformance"], logitem.Time)
			case "99-openshift-artifacts-collector":
				delta = diffDate(pluginFinishedAt["20-openshift-conformance-validated"], logitem.Time)
			}
			runtimeLogs = append(runtimeLogs, &RuntimeInfoItem{
				Name:  fmt.Sprintf("plugin finished %s", logitem.PluginName),
				Time:  logitem.Time,
				Total: diffDate(pluginStartedAt[logitem.PluginName], logitem.Time),
				Delta: delta,
			})
		}

		// marker: plugin cleaned
		if logitem.Message == "Invoking plugin cleanup" {
			msg := "server finished"
			if _, ok := mapExists[msg]; !ok {
				runtimeLogs = append(runtimeLogs, &RuntimeInfoItem{
					Name:  msg,
					Time:  logitem.Time,
					Total: diffDate(serverStartedAt, logitem.Time),
				})
			}
			mapExists[msg] = exists
		}
	}

	return runtimeLogs
}
