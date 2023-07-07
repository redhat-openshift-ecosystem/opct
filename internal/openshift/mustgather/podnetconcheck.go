package mustgather

import log "github.com/sirupsen/logrus"

/* MustGather PodNetworkChecks handle connectivity monitor */

type MustGatherPodNetworkCheck struct {
	Name          string
	SpecSource    string
	SpecTarget    string
	TotalFailures int64
	TotalOutages  int64
	TotalSuccess  int64
}

type MustGatherPodNetworkChecks struct {
	TotalFailures int64
	TotalOutages  int64
	TotalSuccess  int64
	Checks        []*MustGatherPodNetworkCheck
	Outages       []*NetworkOutage
	Failures      []*NetworkCheckFailure
}

func (p *MustGatherPodNetworkChecks) InsertCheck(
	check *MustGatherPodNetworkCheck,
	failures []*NetworkCheckFailure,
	outages []*NetworkOutage,
) {
	p.Checks = append(p.Checks, check)
	p.Outages = append(p.Outages, outages...)
	p.Failures = append(p.Failures, failures...)
	p.TotalFailures += check.TotalFailures
	p.TotalOutages += check.TotalOutages
	p.TotalSuccess += check.TotalSuccess
}

func (p *MustGatherPodNetworkChecks) Parse(data map[string]interface{}) {

	// TODO#1 use CRD PodNetworkConnectivityCheck and api controlplane.operator.openshift.io/v1alpha1 to parse
	// TODO#2 use reflection to read data
	for _, d := range data["items"].([]interface{}) {
		item := d.(map[interface{}]interface{})

		if item["metadata"] == nil {
			log.Errorf("unable to retrieve pod network check metadata: %v", item["metadata"])
			continue
		}
		metadata := item["metadata"].(map[interface{}]interface{})

		if item["spec"] == nil {
			log.Errorf("unable to retrieve pod network check spec: %v", item["spec"])
			continue
		}
		spec := item["spec"].(map[interface{}]interface{})

		if item["status"] == nil {
			log.Errorf("unable to retrieve pod network check status: %v", item["status"])
			continue
		}
		status := item["status"].(map[interface{}]interface{})

		name := metadata["name"].(string)
		check := &MustGatherPodNetworkCheck{
			Name:       name,
			SpecSource: spec["sourcePod"].(string),
			SpecTarget: spec["targetEndpoint"].(string),
		}
		if status["successes"] != nil {
			check.TotalSuccess = int64(len(status["successes"].([]interface{})))
		}

		netFailures := []*NetworkCheckFailure{}
		if status["failures"] != nil {
			failures := status["failures"].([]interface{})
			check.TotalFailures = int64(len(failures))
			for _, f := range failures {
				if f.(map[interface{}]interface{})["time"] == nil {
					continue
				}
				nf := &NetworkCheckFailure{
					Name: name,
					Time: f.(map[interface{}]interface{})["time"].(string),
				}
				if f.(map[interface{}]interface{})["latency"] != nil {
					nf.Latency = f.(map[interface{}]interface{})["latency"].(string)
				}
				if f.(map[interface{}]interface{})["reason"] != nil {
					nf.Reason = f.(map[interface{}]interface{})["reason"].(string)
				}
				if f.(map[interface{}]interface{})["message"] != nil {
					nf.Message = f.(map[interface{}]interface{})["message"].(string)
				}
				netFailures = append(netFailures, nf)
			}
		}

		netOutages := []*NetworkOutage{}
		if status["outages"] != nil {
			outages := status["outages"].([]interface{})
			check.TotalOutages = int64(len(outages))
			for _, o := range outages {
				no := &NetworkOutage{Name: name}
				if o.(map[interface{}]interface{})["start"] == nil {
					continue
				}
				no.Start = o.(map[interface{}]interface{})["start"].(string)
				if o.(map[interface{}]interface{})["end"] != nil {
					no.End = o.(map[interface{}]interface{})["end"].(string)
				}
				if o.(map[interface{}]interface{})["message"] != nil {
					no.Message = o.(map[interface{}]interface{})["message"].(string)
				}
				netOutages = append(netOutages, no)
			}
		}
		p.InsertCheck(check, netFailures, netOutages)
	}

}

type NetworkOutage struct {
	Start   string
	End     string
	Name    string
	Message string
}

type NetworkCheckFailure struct {
	Time    string
	Reason  string
	Latency string
	Name    string
	Message string
}
