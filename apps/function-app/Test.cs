using System;
using System.Text.Json;
using Microsoft.Azure.WebJobs;
using Microsoft.Azure.WebJobs.Host;
using Microsoft.Extensions.Logging;

namespace function_app
{
    public class Test
    {
        [FunctionName("Test")]
        [return: ServiceBus("out", Connection = "sb_conn")]
        public string Run([ServiceBusTrigger("in", Connection = "ConnectionStrings:sb_conn")]string myQueueItem, ILogger log)
        {
            log.LogInformation($"C# ServiceBus queue trigger function processed message");
            return JsonSerializer.Serialize(myQueueItem);
        }
    }
}
