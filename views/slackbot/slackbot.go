package slackbot

import (
	"bytes"
	"fmt"
	"github.com/couchbaselabs/cloud-monitoring-tool/monitoring"
	"github.com/slack-go/slack"
	"log"
	"os"
	"strings"
	"time"
)

const slackBotTokenEnv = "SLACK_BOT_TOKEN"
const slackChannelIdEnv = "SLACK_CHANNEL_ID"

const dateLayout = "1 Jan, 2006 at 3:04pm (UTC)"
const throttleDuration = time.Second

type CloudMonitoringSlackBot struct {
	GlobalCloudContext *monitoring.GlobalCloudContext
}

func (bot *CloudMonitoringSlackBot) PostThreadedReport() error {
	slackToken := os.Getenv(slackBotTokenEnv)

	if slackToken == "" {
		return fmt.Errorf("unable to start Slack bot, %s environment variable not found", slackBotTokenEnv)
	}

	slackChannel := os.Getenv(slackChannelIdEnv)

	if slackChannel == "" {
		return fmt.Errorf("unable to start Slack bot, %s environment variable not found", slackChannelIdEnv)
	}

	if bot.GlobalCloudContext == nil {
		return fmt.Errorf("unable to post messages to Slack, no cloud context found")
	}

	var couchbaseClouds []monitoring.CouchbaseCloud
	var couchbaseCloudClusters []monitoring.CouchbaseCloudCluster
	var cloudformationStacks []monitoring.CloudformationStack
	var eksClusters []monitoring.EKSCluster
	var ec2Instances []monitoring.EC2Instance
	var ebsVolumes []monitoring.EBSVolume

	for _, regionalCtx := range bot.GlobalCloudContext.RegionalCloudContexts {
		for _, couchbaseCloud := range regionalCtx.CouchbaseClouds {
			couchbaseClouds = append(couchbaseClouds, *couchbaseCloud)
		}

		for _, couchbaseCluster := range regionalCtx.CouchbaseCloudClusters {
			couchbaseCloudClusters = append(couchbaseCloudClusters, *couchbaseCluster)
		}

		deepCouchbaseClusters := findCouchbaseCloudClusters(regionalCtx.CouchbaseClouds)
		couchbaseCloudClusters = append(couchbaseCloudClusters, deepCouchbaseClusters...)

		for _, cloudformationStack := range regionalCtx.CloudFormationStacks {
			cloudformationStacks = append(cloudformationStacks, cloudformationStack)
		}

		for _, eksCluster := range regionalCtx.EKSClusters {
			eksClusters = append(eksClusters, eksCluster)
		}

		for _, ec2Instance := range regionalCtx.EC2Instances {
			ec2Instances = append(ec2Instances, ec2Instance)
		}

		for _, ebsVolume := range regionalCtx.EBSVolumes {
			ebsVolumes = append(ebsVolumes, ebsVolume)
		}
	}

	client := slack.New(slackToken)

	header := getReportHeaderBlocks()
	couchbaseCloudBlocks := getCouchbaseCloudParentBlocks(couchbaseClouds)
	couchbaseCloudClusterBlocks := getCouchbaseCloudClusterParentBlocks(couchbaseCloudClusters)
	cloudformationBlocks := getCloudformationParentBlocks(cloudformationStacks)
	eksBlocks := getEKSParentBlocks(eksClusters)
	ec2Blocks := getEC2ParentBlocks(ec2Instances)
	ebsBlocks := getEBSParentBlocks(ebsVolumes)

	_, err := sendSlackGroupMessage(client, slackChannel, header)

	if err != nil {
		return handleSlackMessageError(err)
	}

	couchbaseCloudBlocksTs, err := sendSlackGroupMessage(client, slackChannel, couchbaseCloudBlocks)

	if err != nil {
		return handleSlackMessageError(err)
	}
	couchbaseCloudClusterBlocksTs, err := sendSlackGroupMessage(client, slackChannel, couchbaseCloudClusterBlocks)

	if err != nil {
		return handleSlackMessageError(err)
	}
	cloudformationBlocksTs, err := sendSlackGroupMessage(client, slackChannel, cloudformationBlocks)

	if err != nil {
		return handleSlackMessageError(err)
	}
	eksBlocksTs, err := sendSlackGroupMessage(client, slackChannel, eksBlocks)

	if err != nil {
		return handleSlackMessageError(err)
	}

	ec2BlocksTs, err := sendSlackGroupMessage(client, slackChannel, ec2Blocks)

	if err != nil {
		return handleSlackMessageError(err)
	}

	ebsBlocksTs, err := sendSlackGroupMessage(client, slackChannel, ebsBlocks)

	if err != nil {
		return handleSlackMessageError(err)
	}

	sendCouchbaseCloudReplies(client, slackChannel, couchbaseClouds, couchbaseCloudBlocksTs)
	sendCouchbaseCloudClusterReplies(client, slackChannel, couchbaseCloudClusters, couchbaseCloudClusterBlocksTs)
	sendCloudformationStackReplies(client, slackChannel, cloudformationStacks, cloudformationBlocksTs)
	sendEKSClusterReplies(client, slackChannel, eksClusters, eksBlocksTs)
	sendEC2InstancesReplies(client, slackChannel, ec2Instances, ec2BlocksTs)
	sendEBSInstancesReplies(client, slackChannel, ebsVolumes, ebsBlocksTs)

	return nil
}

func sendSlackGroupMessage(client *slack.Client, channelId string, blocks []slack.Block) (string, error) {
	_, timestamp, _, err := client.SendMessage(channelId, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		return "", err
	}

	return timestamp, nil
}

func getReportHeaderBlocks() []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf("Below is a *cascading* report of all of our cloud infrastructure in AWS. If you have a cloud resource in the below list please take the time to consider if it is currently being used or will be used again today. If the answer is no, please delete the resource.\n\nIf you do have a need to keep a resource please try and ensure you are using as few resources as possible!\n")))
	return blocks
}

func getCouchbaseCloudParentBlocks(couchbaseClouds []monitoring.CouchbaseCloud) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackDividerBlock())
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf("\n\n:thought_balloon:  *Couchbase Clouds* (%d)", len(couchbaseClouds))))
	return blocks
}

func getCouchbaseCloudClusterParentBlocks(couchbaseClusters []monitoring.CouchbaseCloudCluster) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackDividerBlock())
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf(":snow_cloud:  *Couchbase Cloud Clusters* (%d)", len(couchbaseClusters))))
	return blocks
}

func getCloudformationParentBlocks(cloudformationStacks []monitoring.CloudformationStack) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackDividerBlock())
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf(":dango:  *Cloudformation Stacks* (%d)", len(cloudformationStacks))))
	return blocks
}

func getEKSParentBlocks(eksClusters []monitoring.EKSCluster) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackDividerBlock())
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf(":dizzy:  *EKS Clusters* (%d)", len(eksClusters))))
	return blocks
}

func getEC2ParentBlocks(ec2Instances []monitoring.EC2Instance) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackDividerBlock())
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf(":zap:  *EC2 Instances* (%d)", len(ec2Instances))))
	return blocks
}

func getEBSParentBlocks(ebsVolumes []monitoring.EBSVolume) []slack.Block {
	var blocks []slack.Block
	blocks = append(blocks, getSlackDividerBlock())
	blocks = append(blocks, getSlackSectionBlock(fmt.Sprintf(":orange_book:  *EBS Volumes* (%d)", len(ebsVolumes))))
	return blocks
}

func handleSlackMessageError(err error) error {
	return fmt.Errorf("unable to post messages to Slack: %s", err)
}

func getSlackSectionBlock(text string) *slack.SectionBlock {
	return slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil)
}

func getSlackDividerBlock() *slack.DividerBlock {
	return slack.NewDividerBlock()
}

func findCouchbaseCloudClusters(couchbaseClouds map[string]*monitoring.CouchbaseCloud) []monitoring.CouchbaseCloudCluster {
	var couchbaseClusters []monitoring.CouchbaseCloudCluster

	for _, cloud := range couchbaseClouds {
		for _, eksCluster := range cloud.EKSClusters {
			for _, couchbaseCluster := range eksCluster.CouchbaseCloudClusters {
				couchbaseClusters = append (couchbaseClusters, couchbaseCluster)
			}
		}
	}

	return couchbaseClusters
}

func sendCouchbaseCloudReplies(client *slack.Client, channelId string, couchbaseClouds []monitoring.CouchbaseCloud, timestamp string) {
	log.Println("Sending throttled slack replies for Couchbase Cloud")
	for _, cloud := range couchbaseClouds {
		var message bytes.Buffer
		message.WriteString(fmt.Sprintf("*Name*: `%s`\n", cloud.Name))
		message.WriteString(fmt.Sprintf("*Provider*: `%s`\n", cloud.Provider))
		message.WriteString(fmt.Sprintf("*Region*: `%s`\n", cloud.Region))
		message.WriteString(fmt.Sprintf("*Virtual Network CIDR*: `%s`\n", cloud.VirtualNetworkCIDR))
		message.WriteString(fmt.Sprintf("*EKS clusters*: `%d`\n", len(cloud.EKSClusters)))
		message.WriteString(fmt.Sprintf("*Status*: `%s`\n", cloud.Status))

		if err := sendSlackReply(client, channelId, timestamp, message.String()); err != nil {
			log.Printf("Unable to send Slack reply: %s", err)
		}
	}
}

func sendCouchbaseCloudClusterReplies(client *slack.Client, channelId string, couchbaseCloudClusters []monitoring.CouchbaseCloudCluster, timestamp string) {
	log.Println("Sending throttled slack replies for Couchbase Cloud clusters")
	for _, cluster := range couchbaseCloudClusters {
		var message bytes.Buffer
		message.WriteString(fmt.Sprintf("*Name*: `%s`\n", cluster.Name))
		message.WriteString(fmt.Sprintf("*Node Count*: `%d`\n", cluster.NodeCount))
		message.WriteString(fmt.Sprintf("*Services*: `%s`\n", strings.Join(cluster.Services, ", ")))

		if err := sendSlackReply(client, channelId, timestamp, message.String()); err != nil {
			log.Printf("Unable to send Slack reply: %s", err)
		}
	}
}

func sendCloudformationStackReplies(client *slack.Client, channelId string, cloudformationStacks []monitoring.CloudformationStack, timestamp string) {
	log.Println("Sending throttled slack replies for Cloudformation stacks")
	for _, cloudformationStack := range cloudformationStacks {
		var message bytes.Buffer
		if cloudformationStack.Name != "" {
			message.WriteString(fmt.Sprintf("*Name*: `%s`\n", cloudformationStack.Name))
		} else {
			message.WriteString(fmt.Sprintf("*ID*: `%s`\n", cloudformationStack.ID))

		}

		message.WriteString(fmt.Sprintf("*Region*: `%s`\n", cloudformationStack.Region))
		message.WriteString(fmt.Sprintf("*Resource Count*: `%d`\n", len(cloudformationStack.StackResourceList)))
		message.WriteString(fmt.Sprintf("*Age*: `%s`\n", cloudformationStack.CreationDuration))

		if len(cloudformationStack.EC2Instances) > 0 {
			message.WriteString(fmt.Sprintf("*EC2 Instances*: `%d`\n", len(cloudformationStack.EC2Instances)))
		}

		message.WriteString(fmt.Sprintf("*Created*: `%s`\n", cloudformationStack.CreatedAt.UTC().Format(dateLayout)))

		if err := sendSlackReply(client, channelId, timestamp, message.String()); err != nil {
			log.Printf("Unable to send Slack reply: %s", err)
		}
	}
}

func sendEKSClusterReplies(client *slack.Client, channelId string, eksClusters []monitoring.EKSCluster, timestamp string) {
	log.Println("Sending throttled slack replies for EKS clusters")
	for _, eksCluster := range eksClusters {
		var message bytes.Buffer
		message.WriteString(fmt.Sprintf("*Name*: `%s`\n", eksCluster.Name))
		message.WriteString(fmt.Sprintf("*Worker Nodes*: `%d`\n", len(eksCluster.EC2Instances)))
		message.WriteString(fmt.Sprintf("*Subnets*: `%d`\n", len(eksCluster.Subnets)))
		message.WriteString(fmt.Sprintf("*Age*: `%s`\n", eksCluster.Age))
		message.WriteString(fmt.Sprintf("Created: `%s`\n", eksCluster.CreatedAt.UTC().Format(dateLayout)))

		if err := sendSlackReply(client, channelId, timestamp, message.String()); err != nil {
			log.Printf("Unable to send Slack reply: %s", err)
		}
	}
}

func sendEC2InstancesReplies(client *slack.Client, channelId string, ec2Instances []monitoring.EC2Instance, timestamp string) {
	log.Println("Sending throttled slack replies for EC2 instances")
	for _, ec2Instance := range ec2Instances {
		var message bytes.Buffer

		if ec2Instance.Name != "" {
			message.WriteString(fmt.Sprintf("*Name*: `%s`\n", ec2Instance.Name))
		} else {
			message.WriteString(fmt.Sprintf("*ID*: `%s`\n", ec2Instance.ID))
		}

		message.WriteString(fmt.Sprintf("*Region*: `%s`\n", ec2Instance.Region))
		message.WriteString(fmt.Sprintf("*Type*: `%s`\n", ec2Instance.InstanceType))

		if ec2Instance.Platform != "" {
			message.WriteString(fmt.Sprintf("*Platform*: `%s`\n", ec2Instance.Platform))
		}

		if ec2Instance.KeyName != "" {
			message.WriteString(fmt.Sprintf("*Key Name*: `%s`\n", ec2Instance.KeyName))
		}

		message.WriteString(fmt.Sprintf("*Launch Time*: `%s`\n", ec2Instance.CreatedAt.UTC().Format(dateLayout)))

		if err := sendSlackReply(client, channelId, timestamp, message.String()); err != nil {
			log.Printf("Unable to send Slack reply: %s", err)
		}
	}
}

func sendEBSInstancesReplies(client *slack.Client, channelId string, ebsVolumes []monitoring.EBSVolume, timestamp string) {
	log.Println("Sending throttled slack replies for EBS volumes")
	for _, ebsVolume := range ebsVolumes {
		var message bytes.Buffer

		if ebsVolume.Name != "" {
			message.WriteString(fmt.Sprintf("*Name*: `%s`\n", ebsVolume.Name))
		} else {
			message.WriteString(fmt.Sprintf("*ID*: `%s`\n", ebsVolume.ID))
		}

		message.WriteString(fmt.Sprintf("*Region*: `%s`\n", ebsVolume.Region))
		message.WriteString(fmt.Sprintf("*Type*: `%s`\n", *ebsVolume.Type))
		message.WriteString(fmt.Sprintf("*Size GiB*: `%d`\n", ebsVolume.SizeGiB))
		message.WriteString(fmt.Sprintf("*State*: `%s`\n", ebsVolume.State))
		message.WriteString(fmt.Sprintf("*Created*: `%s`\n", ebsVolume.CreatedAt.UTC().Format(dateLayout)))

		if err := sendSlackReply(client, channelId, timestamp, message.String()); err != nil {
			log.Printf("Unable to send Slack reply: %s", err)
		}
	}
}

func sendSlackReply(client *slack.Client, channelId string, timestamp string, text string) error {
	_, _, _, err := client.SendMessage(channelId, slack.MsgOptionCompose(slack.MsgOptionText(text, false), slack.MsgOptionTS(timestamp)))

	if err != nil {
		return err
	}

	time.Sleep(throttleDuration)
	return nil
}
