from diagrams import Cluster, Diagram
from diagrams.aws.compute import (
    EC2Instances
)
from diagrams.aws.network import ELB
from diagrams.k8s.infra import ( Node )

from diagrams.azure.network import DNSZones


with Diagram("OpenShift/OKD Cluster", show=False, filename="./ocp-ha-opct.diagram"):
    dnsApiExt = DNSZones("api.<cluster>.<domain>")
    dnsApsExt = DNSZones("*.apps.<cluster>.<domain>")

    with Cluster("VPC/Network"):

        with Cluster("Public subnets"):
            lbe_api = ELB("LoadBalancer API-Ext")
            lbe_apps = ELB("LoadBalancer Apps-Ext")
        
        with Cluster("Private subnets"):
            lbi_api = ELB("LoadBalancer API-Int")
            dnsApiInt = DNSZones("api-int.<cluster>.<domain>")
            with Cluster("Control Plane Pool"):
                cp_group = [EC2Instances("master-0{1,2,3}")]

            with Cluster("Compute Pool"):
                # wk_group = [EC2("compute-01"),
                #             EC2("compute-02"),
                #             EC2("compute-03")]
                wk_group = [EC2Instances("compute-0{1,2,3}")]

            with Cluster("OPCT Dedicated Node"):
                ded_node = [Node("compute-04")]


    dnsApiExt >> lbe_api >> cp_group
    dnsApiInt >> lbi_api >> cp_group
    cp_group >> dnsApiInt >> lbi_api
    wk_group >> dnsApiInt >> lbi_api
    ded_node >> dnsApiInt >> lbi_api
    dnsApsExt >> lbe_apps >> wk_group
