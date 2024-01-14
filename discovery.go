package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var SYMBOLS = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

func randString(size, base int) string {
	var result strings.Builder
	for i := 0; i < size; i++ {
		result.WriteByte(SYMBOLS[rand.Intn(base)])
	}
	return result.String()
}

func findTagValue(xml, tag string) string {
	log.Printf("Finding value for tag: %s", tag)
	regex := regexp.MustCompile(fmt.Sprintf(`<[^/>]*%s[^>]*>([^<]+)`, tag))
	m := regex.FindStringSubmatch(xml)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func discoveryStreamingDevices() []map[string]string {
	log.Println("Starting device discovery")
	conn, err := net.ListenPacket("udp", "0.0.0.0:0")
	if err != nil {
		log.Fatalf("Error listening on UDP: %v", err)
	}
	defer conn.Close()

	randomUUID := uuid.New().String()
	msg := fmt.Sprintf(`<?xml version="1.0" ?>
	<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">
		<s:Header xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing">
			<a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</a:Action>
			<a:MessageID>urn:uuid:%s</a:MessageID>
			<a:To>urn:schemas-xmlsoap-org:ws:2005:04:discovery</a:To>
		</s:Header>
		<s:Body>
			<d:Probe xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery">
				<d:Types />
				<d:Scopes />
			</d:Probe>
		</s:Body>
	</s:Envelope>`, randomUUID)

	//msg := fmt.Sprintf(`your message with %s`, randomUUID) // Replace with your actual message

	log.Printf("Sending message: %s", msg)

	addr, err := net.ResolveUDPAddr("udp", "239.255.255.250:3702")
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}
	_, err = conn.WriteTo([]byte(msg), addr)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	devices := make([]map[string]string, 0)
	for {
		buffer := make([]byte, 8192)
		_, _, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Println("Finished reading from UDP: ", err)
			break
		}
		decodedData := string(buffer)
		log.Println("Received data: ", decodedData)
		if !strings.Contains(decodedData, "onvif") {
			continue
		}
		urlStr := findTagValue(decodedData, "XAddrs")
		if urlStr == "" {
			continue
		}
		if strings.HasPrefix(urlStr, "http://0.0.0.0") {
			urlStr = "http://" + addr.IP.String() + urlStr[14:]
		}
		urlStr = strings.Split(urlStr, " ")[0]
		parsedURL, _ := url.Parse(urlStr)
		scopes := findTagValue(decodedData, "Scopes")
		hardware := extractValueFromScopes(scopes, "hardware")
		mac := extractValueFromScopes(scopes, "MAC")
		name := extractValueFromScopes(scopes, "name")
		profile := extractProfilesFromScopes(scopes)
		metadataVersion := findTagValue(decodedData, "MetadataVersion")
		device := map[string]string{
			"name":             url.PathEscape(name),
			"hardware":         url.PathEscape(hardware),
			"ip":               fmt.Sprintf("%s:%s", parsedURL.Hostname(), parsedURL.Port()),
			"xaddrs":           urlStr,
			"mac":              url.PathEscape(mac),
			"profile":          strings.Join(profile, ", "), // Converting slice to string
			"metadata_version": url.PathEscape(metadataVersion),
		}
		devices = append(devices, device)
	}
	return devices
}

// Rest of your functions (extractValueFromScopes, extractProfilesFromScopes, extractIP) go here...

func extractValueFromScopes(scopes, key string) string {
	value := ""
	if strings.Contains(scopes, key) {
		value = scopes[strings.Index(scopes, key)+len(key)+1:]
		value = strings.Split(value, " ")[0]
	}
	return value
}

func extractProfilesFromScopes(scopes string) []string {
	profile := make([]string, 0)
	for _, scope := range strings.Split(scopes, " ") {
		if strings.HasPrefix(scope, "onvif://www.onvif.org/Profile") {
			profile = append(profile, strings.Split(scope, "/")[len(strings.Split(scope, "/"))-1])
		}
	}
	return profile
}

func extractIP() string {
	st, _ := net.Dial("udp", "10.255.255.255:1")
	defer st.Close()

	ip := "127.0.0.1"
	if addr, ok := st.LocalAddr().(*net.UDPAddr); ok {
		ip = addr.IP.String()
	}
	return ip
}

func main() {
	log.Println("Main function started")
	devices := discoveryStreamingDevices()
	log.Println("Discovered devices: ", devices)
}
