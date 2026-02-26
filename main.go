package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// client struct to pass the client to the event handler
type MyClient struct {
	WAClient *whatsmeow.Client
	eventHandlerID uint32
}

// register helper
func (myclient *MyClient) register() {
	myclient.eventHandlerID = myclient.WAClient.AddEventHandler(myclient.myEventHandler)
}

// main event handler function
func (myclient *MyClient) myEventHandler(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		// initial vars 
		ctx := context.Background()
		client := myclient.WAClient
		sender := v.Info.Sender
		chat := v.Info.Chat 

		// If this is a broadcast message, skip
		if chat.Server == watypes.BroadcastServer {
			fmt.Println("New status update!")
			fmt.Println()
			return
		}

		// get the phone number of the user because it sometimes passes a jid or lid not a phone number.
		var phoneNumber string
		if sender.Server == watypes.HiddenUserServer {
			pnJID, err := client.Store.LIDs.GetPNForLID(ctx, sender)
			if err == nil && !pnJID.IsEmpty() {
				phoneNumber = pnJID.User
			} else {
				phoneNumber = sender.User
			}
		} else {
			phoneNumber = sender.User
		}

		// get the jid from the phone number (i know it seems backward but it's becasue of lids)
		jid := watypes.NewJID(phoneNumber, types.DefaultUserServer)

		// obtain the contact from the jid
		contact, err := myclient.WAClient.Store.Contacts.GetContact(ctx, jid)

		// get the contact's saved name with backups.
		var savedName string
		if err == nil && contact.Found {
			savedName = contact.FullName
			if savedName == "" {
				savedName = contact.FirstName
			}
			if savedName == "" {
				savedName = contact.PushName
			}
		} else {
			savedName = "Error fetching name"
		}

		// get the message text
		text := v.Message.GetConversation()
		if text == "" {
			text = v.Message.GetExtendedTextMessage().GetText()
		}

		fmt.Println("New Message:", text)
		if sender == chat {
			fmt.Println("From senderPN:", phoneNumber)
		} else {
			fmt.Println("From senderPN:", phoneNumber, "On chat:", chat)
		}
		fmt.Println("Sender name:", savedName) 
		fmt.Println()
		/*
		if phoneNumber == "61486036614" {
			client := myclient.WAClient
			reaction := client.BuildReaction(v.Info.Chat, v.Info.Sender, v.Info.ID, "🐈‍⬛")
			myclient.WAClient.SendMessage(context.Background(), v.Info.Chat, reaction)
		}
		*/
	}
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	ctx := context.Background()

	container, err := sqlstore.New(ctx, "sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)

	}
	// clientLog := waLog.Stdout("Client", "DEBUG", true)
	tempcli := whatsmeow.NewClient(deviceStore, nil)
	clientstruct := MyClient { tempcli, 0 }
	clientstruct.register()
	client := clientstruct.WAClient

	if client.Store.ID == nil {

		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}

