package collectors

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/model"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const integrationResponseLimit = 8 << 20

func (r Runner) integrationRequest(ctx context.Context, method, raw string, body io.Reader, headers http.Header) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, method, raw, body)
	if e != nil {
		return nil, errors.New("invalid provider request")
	}
	req.Header = headers
	client := r.client(20 * time.Second)
	if r.integrationClient != nil {
		client = r.integrationClient()
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return errors.New("integration redirects are not permitted")
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, errors.New("provider unreachable, timed out or destination denied")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider HTTP %d; verify permissions, credential validity and endpoint", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, integrationResponseLimit+1))
	if e != nil {
		return nil, errors.New("failed to read provider response")
	}
	if len(b) > integrationResponseLimit {
		return nil, errors.New("provider response exceeds 8 MiB")
	}
	return b, nil
}
func requiredEnv(prefix, key string) (string, error) {
	v := os.Getenv(prefix + "_" + key)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("missing environment variable %s_%s", prefix, key)
	}
	return v, nil
}
func (r Runner) CollectIntegration(ctx context.Context, p model.Integration) (model.IntegrationResult, error) {
	result := model.IntegrationResult{At: time.Now().UTC(), Status: "ok", Resources: []model.IntegrationResource{}}
	var e error
	switch p.Kind {
	case "aws":
		result.Resources, result.Truncated, e = r.awsInventory(ctx, p)
	case "azure":
		result.Resources, result.Truncated, e = r.azureInventory(ctx, p)
	case "gcp":
		result.Resources, result.Truncated, e = r.gcpInventory(ctx, p)
	case "snmp_exporter":
		result.Resources, result.Truncated, e = r.deviceInventory(ctx, p)
	default:
		e = errors.New("unsupported integration")
	}
	if e != nil {
		result.Status = "error"
		result.Error = e.Error()
		result.Resources = []model.IntegrationResource{}
	}
	return result, e
}
func hashText(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func hmacBytes(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(s))
	return h.Sum(nil)
}
func signEC2(raw string, at time.Time, key, secret, token, region string) http.Header {
	u, _ := url.Parse(raw)
	stamp := at.UTC().Format("20060102T150405Z")
	date := stamp[:8]
	headers := http.Header{}
	headers.Set("X-Amz-Date", stamp)
	headers.Set("X-Amz-Content-Sha256", hashText(""))
	names := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	values := map[string]string{"host": u.Host, "x-amz-date": stamp, "x-amz-content-sha256": hashText("")}
	if token != "" {
		headers.Set("X-Amz-Security-Token", token)
		names = append(names, "x-amz-security-token")
		values["x-amz-security-token"] = token
	}
	sort.Strings(names)
	canonicalHeaders := ""
	for _, n := range names {
		canonicalHeaders += n + ":" + strings.TrimSpace(values[n]) + "\n"
	}
	signed := strings.Join(names, ";")
	query := strings.ReplaceAll(u.Query().Encode(), "+", "%20")
	canonical := "GET\n/\n" + query + "\n" + canonicalHeaders + "\n" + signed + "\n" + hashText("")
	scope := date + "/" + region + "/ec2/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hashText(canonical)
	signing := hmacBytes(hmacBytes(hmacBytes(hmacBytes([]byte("AWS4"+secret), date), region), "ec2"), "aws4_request")
	signature := hex.EncodeToString(hmacBytes(signing, toSign))
	headers.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+key+"/"+scope+", SignedHeaders="+signed+", Signature="+signature)
	return headers
}
func (r Runner) awsInventory(ctx context.Context, p model.Integration) ([]model.IntegrationResource, bool, error) {
	key, e := requiredEnv(p.CredentialPrefix, "ACCESS_KEY_ID")
	if e != nil {
		return nil, false, e
	}
	secret, e := requiredEnv(p.CredentialPrefix, "SECRET_ACCESS_KEY")
	if e != nil {
		return nil, false, e
	}
	token := os.Getenv(p.CredentialPrefix + "_SESSION_TOKEN")
	out := []model.IntegrationResource{}
	next := ""
	for page := 0; page < 10; page++ {
		q := url.Values{"Action": {"DescribeInstances"}, "Version": {"2016-11-15"}, "MaxResults": {"100"}}
		if next != "" {
			q.Set("NextToken", next)
		}
		raw := "https://ec2." + p.Region + ".amazonaws.com/?" + strings.ReplaceAll(q.Encode(), "+", "%20")
		b, e := r.integrationRequest(ctx, "GET", raw, nil, signEC2(raw, time.Now(), key, secret, token, p.Region))
		if e != nil {
			return nil, false, e
		}
		var response struct {
			XMLName      xml.Name
			Next         string `xml:"nextToken"`
			Reservations []struct {
				Instances []struct {
					ID       string `xml:"instanceId"`
					Type     string `xml:"instanceType"`
					IP       string `xml:"privateIpAddress"`
					PublicIP string `xml:"ipAddress"`
					VPC      string `xml:"vpcId"`
					Subnet   string `xml:"subnetId"`
					State    struct {
						Name string `xml:"name"`
					} `xml:"instanceState"`
					Placement struct {
						Zone string `xml:"availabilityZone"`
					} `xml:"placement"`
					Tags []struct {
						Key   string `xml:"key"`
						Value string `xml:"value"`
					} `xml:"tagSet>item"`
				} `xml:"instancesSet>item"`
			} `xml:"reservationSet>item"`
		}
		if xml.Unmarshal(b, &response) != nil || response.XMLName.Local != "DescribeInstancesResponse" {
			return nil, false, errors.New("invalid EC2 XML response")
		}
		for _, reservation := range response.Reservations {
			for _, v := range reservation.Instances {
				if v.ID == "" {
					return nil, false, errors.New("EC2 response missing instance identity")
				}
				name := v.ID
				for _, tag := range v.Tags {
					if tag.Key == "Name" && strings.TrimSpace(tag.Value) != "" {
						name = tag.Value
					}
				}
				out = append(out, model.IntegrationResource{ID: v.ID, Name: name, Kind: "aws.ec2.instance", Location: v.Placement.Zone, Status: v.State.Name, Attributes: map[string]string{"instance_type": v.Type, "private_ip": v.IP, "public_ip": v.PublicIP, "vpc_id": v.VPC, "subnet_id": v.Subnet}})
				if len(out) >= 1000 {
					return out, true, nil
				}
			}
		}
		next = response.Next
		if next == "" {
			return out, false, nil
		}
	}
	return out, true, nil
}
func (r Runner) azureInventory(ctx context.Context, p model.Integration) ([]model.IntegrationResource, bool, error) {
	tenant, e := requiredEnv(p.CredentialPrefix, "TENANT_ID")
	if e != nil {
		return nil, false, e
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9.-]{1,150}$`).MatchString(tenant) {
		return nil, false, errors.New("invalid Azure tenant")
	}
	client, e := requiredEnv(p.CredentialPrefix, "CLIENT_ID")
	if e != nil {
		return nil, false, e
	}
	secret, e := requiredEnv(p.CredentialPrefix, "CLIENT_SECRET")
	if e != nil {
		return nil, false, e
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {client}, "client_secret": {secret}, "scope": {"https://management.azure.com/.default"}}
	b, e := r.integrationRequest(ctx, "POST", "https://login.microsoftonline.com/"+tenant+"/oauth2/v2.0/token", strings.NewReader(form.Encode()), http.Header{"Content-Type": {"application/x-www-form-urlencoded"}})
	if e != nil {
		return nil, false, e
	}
	var auth struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(b, &auth) != nil || auth.AccessToken == "" {
		return nil, false, errors.New("Azure token response missing access token")
	}
	out := []model.IntegrationResource{}
	next := "https://management.azure.com/subscriptions/" + url.PathEscape(p.Scope) + "/resources?api-version=2021-04-01"
	for page := 0; page < 10; page++ {
		u, e := url.Parse(next)
		if e != nil || u.Scheme != "https" || u.Host != "management.azure.com" || u.User != nil || !strings.HasPrefix(u.Path, "/subscriptions/"+p.Scope+"/") {
			return nil, false, errors.New("invalid Azure pagination URL")
		}
		b, e := r.integrationRequest(ctx, "GET", next, nil, http.Header{"Authorization": {"Bearer " + auth.AccessToken}})
		if e != nil {
			return nil, false, e
		}
		var response struct {
			Value []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Type     string `json:"type"`
				Location string `json:"location"`
			} `json:"value"`
			Next string `json:"nextLink"`
		}
		if json.Unmarshal(b, &response) != nil || response.Value == nil {
			return nil, false, errors.New("invalid Azure resources response")
		}
		for _, v := range response.Value {
			if v.ID == "" {
				return nil, false, errors.New("Azure resource identity missing")
			}
			out = append(out, model.IntegrationResource{ID: v.ID, Name: v.Name, Kind: v.Type, Location: v.Location, Status: "observed"})
			if len(out) >= 1000 {
				return out, true, nil
			}
		}
		next = response.Next
		if next == "" {
			return out, false, nil
		}
	}
	return out, true, nil
}
func (r Runner) gcpInventory(ctx context.Context, p model.Integration) ([]model.IntegrationResource, bool, error) {
	token, e := requiredEnv(p.CredentialPrefix, "ACCESS_TOKEN")
	if e != nil {
		return nil, false, e
	}
	out := []model.IntegrationResource{}
	next := ""
	for page := 0; page < 10; page++ {
		q := url.Values{"maxResults": {"100"}, "returnPartialSuccess": {"false"}}
		if next != "" {
			q.Set("pageToken", next)
		}
		b, e := r.integrationRequest(ctx, "GET", "https://compute.googleapis.com/compute/v1/projects/"+url.PathEscape(p.Scope)+"/aggregated/instances?"+q.Encode(), nil, http.Header{"Authorization": {"Bearer " + token}})
		if e != nil {
			return nil, false, e
		}
		var response struct {
			Next  string `json:"nextPageToken"`
			Items map[string]struct {
				Warning *struct {
					Code string `json:"code"`
				} `json:"warning"`
				Instances []struct {
					ID                json.Number `json:"id"`
					Name              string      `json:"name"`
					SelfLink          string      `json:"selfLink"`
					Status            string      `json:"status"`
					Zone              string      `json:"zone"`
					MachineType       string      `json:"machineType"`
					NetworkInterfaces []struct {
						NetworkIP string `json:"networkIP"`
						Network   string `json:"network"`
					} `json:"networkInterfaces"`
				} `json:"instances"`
			} `json:"items"`
		}
		if json.Unmarshal(b, &response) != nil {
			return nil, false, errors.New("invalid GCP instances response")
		}
		keys := []string{}
		for k := range response.Items {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			scope := response.Items[k]
			if scope.Warning != nil && scope.Warning.Code != "NO_RESULTS_ON_PAGE" {
				return nil, false, errors.New("GCP reported partial inventory; collection not complete")
			}
			for _, v := range scope.Instances {
				if v.Name == "" || v.Zone == "" {
					return nil, false, errors.New("GCP instance identity missing")
				}
				id := v.SelfLink
				if id == "" {
					id = v.Zone + "/instances/" + v.Name
				}
				attrs := map[string]string{"instance_id": string(v.ID), "machine_type": v.MachineType}
				if len(v.NetworkInterfaces) > 0 {
					attrs["private_ip"] = v.NetworkInterfaces[0].NetworkIP
					attrs["network"] = v.NetworkInterfaces[0].Network
				}
				out = append(out, model.IntegrationResource{ID: id, Name: v.Name, Kind: "gcp.compute.instance", Location: v.Zone, Status: v.Status, Attributes: attrs})
				if len(out) >= 1000 {
					return out, true, nil
				}
			}
		}
		next = response.Next
		if next == "" {
			return out, false, nil
		}
	}
	return out, true, nil
}
