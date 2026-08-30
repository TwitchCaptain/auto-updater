//go:build windows

package toast

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func show(n Note) error {
	if n.AppID == "" {
		n.AppID = "TwitchCaptain.CaptainUpdater"
	}

	xml := fmt.Sprintf(`<toast scenario="reminder" activationType="protocol" launch="%s">
  <visual>
    <binding template="ToastGeneric">
      <text>%s</text>
      <text>%s</text>
    </binding>
  </visual>
  <actions>
    <action content="Open" activationType="protocol" arguments="%s"/>
  </actions>
</toast>`, xmlEsc(n.URL), xmlEsc(n.Title), xmlEsc(n.Message), xmlEsc(n.URL))

	ps := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml(@'
%s
'@)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
$toast.ExpirationTime = [DateTimeOffset]::Now.AddDays(2)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('%s').Show($toast)
`, xml, n.AppID)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	return cmd.Run()
}

func xmlEsc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")

	return s
}
