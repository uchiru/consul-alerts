package notifier

import (
    "fmt"
    "strings"

    "github.com/uchiru/consul-alerts/Godeps/_workspace/src/github.com/opsgenie/opsgenie-go-sdk/alertsv2"
    ogcli "github.com/uchiru/consul-alerts/Godeps/_workspace/src/github.com/opsgenie/opsgenie-go-sdk/client"

    log "github.com/uchiru/consul-alerts/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

type OpsGenieNotifier struct {
    Enabled     bool
    ClusterName string `json:"cluster-name"`
    ApiKey      string `json:"api-key"`
    ApiUrl      string `json:"api-url"`
}

// NotifierName provides name for notifier selection
func (opsgenie *OpsGenieNotifier) NotifierName() string {
    return "opsgenie"
}

func (opsgenie *OpsGenieNotifier) Copy() Notifier {
    notifier := *opsgenie
    return &notifier
}


//Notify sends messages to the endpoint notifier
func (opsgenie *OpsGenieNotifier) Notify(messages Messages) bool {

    overallStatus, pass, warn, fail := messages.Summary()

    client := new(ogcli.OpsGenieClient)
    client.SetAPIKey(opsgenie.ApiKey)
    client.SetOpsGenieAPIUrl(opsgenie.ApiUrl)
    log.Info(fmt.Sprintf("ApiUrl is: %s and Key: %s", client.OpsGenieAPIUrl(), client.APIKey()))
    alertCli, cliErr := client.AlertV2()

    if cliErr != nil {
        log.Println("Opsgenie notification trouble with client")
        return false
    }

    ok := true
    for _, message := range messages {
        // title := fmt.Sprintf("\n%s:%s:%s is %s.", message.Node, message.Service, message.Check, message.Status)
        title := fmt.Sprintf("\n[%s] %s=>%s=>%s %s", opsgenie.ClusterName, strings.ToUpper(message.Status), message.Node, strings.Replace(message.Service, "-main", "", 3), message.Output)
        alias := opsgenie.createAlias(message)
        content := fmt.Sprintf(header, opsgenie.ClusterName, overallStatus, fail, warn, pass)
        content += fmt.Sprintf("\n%s:%s:%s is %s.", message.Node, message.Service, message.Check, message.Status)
        content += fmt.Sprintf("\n%s", message.Output)

        // create the alert
        switch {
        case message.IsCritical():
            ok = opsgenie.createAlert(alertCli, title, content, alias) && ok
        case message.IsWarning():
            ok = opsgenie.createAlert(alertCli, title, content, alias) && ok
        case message.IsPassing():
            ok = opsgenie.closeAlert(alertCli, alias) && ok
        default:
            ok = false
            log.Warn("Message was not either IsCritical, IsWarning or IsPasssing. No notification was sent for ", alias)
        }
    }
    return ok
}

func (opsgenie OpsGenieNotifier) createAlias(message Message) string {
    incidentKey := message.Node
    if message.ServiceId != "" {
        incidentKey += ":" + message.ServiceId
    }

    return incidentKey
}

func (opsgenie *OpsGenieNotifier) createAlert(alertCli *ogcli.OpsGenieAlertV2Client, message string, content string, alias string) bool {
    log.Debug(fmt.Sprintf("OpsGenieAlertClient.CreateAlert alias: %s", alias))

    req := alertsv2.CreateAlertRequest{
        Message:     message,
        Description: content,
        Alias:       alias,
        Source:      "consul",
        Entity:      opsgenie.ClusterName,
        Tags:        message.ServiceTags
    }
    response, alertErr := alertCli.Create(req)

    if alertErr != nil {
        if response == nil {
            log.Warn("Opsgenie notification trouble. ", alertErr)
        } else {
            log.Warn("Opsgenie notification trouble. ", response.RequestID)
        }
        return false
    }

    log.Println("Opsgenie notification sent.")
    return true
}

func (opsgenie *OpsGenieNotifier) closeAlert(alertCli *ogcli.OpsGenieAlertV2Client, alias string) bool {
    log.Debug(fmt.Sprintf("OpsGenieAlertClient.CloseAlert alias: %s", alias))

    identifier := alertsv2.Identifier{
        Alias: alias,
    }

    req := alertsv2.CloseRequest{
        Identifier: &identifier,
        Source:     "consul",
    }
    response, alertErr := alertCli.Close(req)

    if alertErr != nil {
        if response == nil {
            log.Warn("Opsgenie notification trouble. ", alertErr)
        } else {
            log.Warn("Opsgenie notification trouble. ", response.RequestID)
        }
        return false
    }

    log.Println("Opsgenie close alert sent.")
    return true
}
