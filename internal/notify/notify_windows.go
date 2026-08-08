//go:build windows

package notify

import (
	"encoding/xml"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"sync"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
	"git.sr.ht/~jackmordaunt/go-toast/v2/wintoast"
)

const (
	toastAppID        = "Vocab"
	toastActivation   = "foreground"
	toastDuration     = "short"
	toastDefaultAudio = "ms-winsoundevent:Notification.Default"
)

type toastRequest struct {
	xml    string
	result chan error
}

var toastWorker struct {
	once  sync.Once
	queue chan toastRequest
}

type toastPayload struct {
	XMLName        xml.Name      `xml:"toast"`
	ActivationType string        `xml:"activationType,attr"`
	Launch         string        `xml:"launch,attr,omitempty"`
	Duration       string        `xml:"duration,attr"`
	Visual         toastVisual   `xml:"visual"`
	Audio          toastAudio    `xml:"audio"`
	Actions        *toastActions `xml:"actions,omitempty"`
}

type toastVisual struct {
	Binding toastBinding `xml:"binding"`
}

type toastBinding struct {
	Template string      `xml:"template,attr"`
	Texts    []toastText `xml:"text"`
}

type toastText struct {
	Value string `xml:",chardata"`
}

type toastAudio struct {
	Source string `xml:"src,attr"`
}

type toastActions struct {
	Actions []toastAction `xml:"action"`
}

type toastAction struct {
	Type      string `xml:"activationType,attr"`
	Content   string `xml:"content,attr"`
	Arguments string `xml:"arguments,attr"`
}

func startToastWorker() {
	toastWorker.once.Do(func() {
		toastWorker.queue = make(chan toastRequest)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			for request := range toastWorker.queue {
				err := wintoast.Push(toastAppID, request.xml)
				if err != nil {
					log.Printf("notify: publish toast: %v", err)
				}
				request.result <- err
			}
		}()
	})
}

func publishToast(payload toastPayload) error {
	data, err := xml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal toast: %w", err)
	}
	startToastWorker()
	result := make(chan error, 1)
	toastWorker.queue <- toastRequest{xml: xml.Header + string(data), result: result}
	return <-result
}

func RegisterApp(exePath string) error {
	return toast.SetAppData(toast.AppData{
		AppID:         toastAppID,
		ActivationExe: exePath,
		IconPath:      filepath.Join(filepath.Dir(exePath), "icon.ico"),
	})
}

func setActivationCallback(callback func(arguments string)) {
	toast.SetActivationCallback(func(arguments string, _ []toast.UserData) {
		log.Printf("notify: activation received: %q", arguments)
		callback(arguments)
	})
}

func Send(w Word) error {
	return publishToast(toastPayload{
		ActivationType: toastActivation,
		Duration:       toastDuration,
		Visual:         makeVisual("Do you remember the meaning of \"" + w.Text + "\"?"),
		Audio:          toastAudio{Source: toastDefaultAudio},
		Actions: &toastActions{Actions: []toastAction{
			{Type: toastActivation, Content: "Knew it", Arguments: fmt.Sprintf("--review %d --rating 2", w.ID)},
			{Type: toastActivation, Content: "Struggled", Arguments: fmt.Sprintf("--review %d --rating 1", w.ID)},
			{Type: toastActivation, Content: "Forgot", Arguments: fmt.Sprintf("--review %d --rating 0", w.ID)},
		}},
	})
}

func sendStatus(message string) error {
	return publishToast(toastPayload{
		ActivationType: toastActivation,
		Duration:       toastDuration,
		Visual:         makeVisual(message),
		Audio:          toastAudio{Source: toastDefaultAudio},
	})
}

func sendProduction(w Word) error {
	return publishToast(toastPayload{
		ActivationType: toastActivation,
		Duration:       toastDuration,
		Visual:         makeVisual("Can you use \"" + w.Text + "\" in a sentence?"),
		Audio:          toastAudio{Source: toastDefaultAudio},
		Actions: &toastActions{Actions: []toastAction{
			{Type: toastActivation, Content: "Got it", Arguments: fmt.Sprintf("--produce %d --produced", w.ID)},
			{Type: toastActivation, Content: "Couldn't", Arguments: fmt.Sprintf("--produce %d", w.ID)},
		}},
	})
}

func makeVisual(body string) toastVisual {
	return toastVisual{Binding: toastBinding{
		Template: "ToastGeneric",
		Texts:    []toastText{{Value: "Vocab"}, {Value: body}},
	}}
}
