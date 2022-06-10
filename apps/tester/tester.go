package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appservice/armappservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/servicebus/armservicebus"
	"github.com/schollz/progressbar/v3"
)

var (
	sbConn         string = "<connection_string>"
	sbNamespace    string = "<namespace_name>"
	resourceGroup  string = "sb-functions-throughput-test"
	functionApp    string = "sb-functions-throughput-test"
	subscriptionId string = "<subscription_name>"
	webAppsClient  *armappservice.WebAppsClient
	queueClient    *armservicebus.QueuesClient
	sbClient       *azservicebus.Client
	mu             sync.Mutex
	sentCount      int
	inQueueName    string = "in"
	outQueueName   string = "out"
	message        []byte
	sendBatchSize  int = 5
	messageCount   int = 5000
)

func main() {
	initClients()
	stopFunctionApp()
	getTestMessage()
	recreateQueues([]string{inQueueName, outQueueName})
	queueStats()
	sendMessages()
	startFunctionApp()
	var startTime time.Time

	for {
		inCount, outCount := queueStats()
		if outCount > 0 && startTime.IsZero() {
			startTime = time.Now()
		}
		if inCount == 0 && outCount >= int64(messageCount) {
			break
		}
		time.Sleep(time.Second)
	}

	endTime := time.Now()
	fmt.Println("Time taken: ", endTime.Sub(startTime).Seconds())
}

func initClients() {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		panic(err)
	}

	webAppsClient, err = armappservice.NewWebAppsClient(subscriptionId, cred, nil)
	if err != nil {
		panic(err)
	}

	queueClient, err = armservicebus.NewQueuesClient(subscriptionId, cred, nil)
	if err != nil {
		panic(err)
	}

	sbClient, err = azservicebus.NewClientFromConnectionString(sbConn, nil)
	if err != nil {
		panic(err)
	}
}

func queueStats() (inCount int64, outCount int64) {
	inChan := make(chan *armservicebus.SBQueue)
	outChan := make(chan *armservicebus.SBQueue)
	go getQueue(queueClient, inQueueName, inChan)
	go getQueue(queueClient, outQueueName, outChan)
	inQueue := <-inChan
	outQueue := <-outChan
	fmt.Println("In queue count: ", strconv.FormatInt(*inQueue.Properties.MessageCount, 10), " Out queue count: ", strconv.FormatInt(*outQueue.Properties.MessageCount, 10))
	return *inQueue.Properties.MessageCount, *outQueue.Properties.MessageCount
}

func getQueue(queueClient *armservicebus.QueuesClient, queueName string, ch chan *armservicebus.SBQueue) {
	queue, err := queueClient.Get(context.Background(), resourceGroup, sbNamespace, queueName, nil)
	if err != nil {
		panic(err)
	}
	ch <- &queue.SBQueue
}

func getTestMessage() {
	file, err := ioutil.ReadFile("testmessage.json")
	if err != nil {
		panic(err)
	}
	message = file
}

func sendMessages() {
	sender, err := sbClient.NewSender(inQueueName, nil)
	if err != nil {
		panic(err)
	}

	wg := &sync.WaitGroup{}
	bar := progressbar.Default(int64(messageCount), "Sending messages")

	for i := 0; i < (messageCount / sendBatchSize); i++ {
		wg.Add(1)
		go sendMessage(sender, wg, bar)
	}

	wg.Wait()
}

func sendMessage(sender *azservicebus.Sender, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	defer wg.Done()
	batch := newMessageBatch(sender)
	err := sender.SendMessageBatch(context.Background(), batch, nil)
	if err != nil {
		fmt.Println("error sending message: ", err)
	}
	incrementProgressBar(bar, sendBatchSize)
}

func newMessageBatch(sender *azservicebus.Sender) *azservicebus.MessageBatch {
	batch, err := sender.NewMessageBatch(context.Background(), nil)
	if err != nil {
		panic(err)
	}

	for i := 0; i < sendBatchSize; i++ {
		m := &azservicebus.Message{
			Body: message,
		}
		err := batch.AddMessage(m, nil)
		if err != nil {
			panic(err)
		}
	}
	return batch
}

func incrementProgressBar(bar *progressbar.ProgressBar, batchSize int) {
	mu.Lock()
	defer mu.Unlock()
	bar.Add(batchSize)
}

func recreateQueues(queues []string) {
	wg := &sync.WaitGroup{}
	for _, queue := range queues {
		wg.Add(1)
		go recreateQueue(queue, wg)
	}
	wg.Wait()
}

func recreateQueue(name string, wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Printf("Recreating queue %s\n", name)

	_, err := queueClient.Delete(context.Background(), resourceGroup, sbNamespace, name, nil)
	if err != nil {
		panic(err)
	}
	_, err = queueClient.CreateOrUpdate(context.Background(), resourceGroup, sbNamespace, name, armservicebus.SBQueue{}, nil)
	if err != nil {
		panic(err)
	}
}

func stopFunctionApp() {
	fmt.Println("Stopping function app...")
	_, err := webAppsClient.Stop(context.Background(), resourceGroup, functionApp, nil)
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 20)
}

func startFunctionApp() {
	fmt.Println("Starting function app...")
	_, err := webAppsClient.Start(context.Background(), resourceGroup, functionApp, nil)
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 20)
}
