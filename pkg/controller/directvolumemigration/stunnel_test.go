package directvolumemigration

import (
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	migapi "github.com/konveyor/mig-controller/pkg/apis/migration/v1alpha1"
	"github.com/konveyor/mig-controller/pkg/settings"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var justHostProxy = "https://just-host"

var mangledHostProxy = "https://@valid"

var validProxySecret = "https://user:pass@ip:1111"

var basicAuthWithSpecialChars = "https://user:!:pass@ip:1111"

var invalidProxy = "https://220.22.23.265.253.255:err"

func TestTask_generateStunnelProxyConfig(t *testing.T) {
	type fields struct {
		Log              logr.Logger
		Client           k8sclient.Client
		Owner            *migapi.DirectVolumeMigration
		SSHKeys          *sshKeys
		RsyncRoutes      map[string]string
		Phase            string
		PhaseDescription string
		PlanResources    *migapi.PlanResources
		MigrationUID     string
		Requeue          time.Duration
		Itinerary        Itinerary
		Errors           []string
	}
	tests := []struct {
		name        string
		fields      fields
		proxyString string
		want        stunnelProxyConfig
		wantErr     bool
	}{
		{
			name:        "when given a proxy without username/pass, should return proxy config with host without username/pass",
			proxyString: justHostProxy,
			want: stunnelProxyConfig{
				ProxyHost: "just-host",
			},
			fields: fields{
				Log: log.Real,
			},
			wantErr: false,
		},
		{
			name:        "when given a proxy with mangled host string, should return proxy config with host without username/pass",
			proxyString: mangledHostProxy,
			want: stunnelProxyConfig{
				ProxyHost: "valid",
			},
			fields: fields{
				Log: log.Real,
			},
			wantErr: false,
		},
		{
			name: "when given a valid proxy with username/pass, should proxy config with username/pass",
			want: stunnelProxyConfig{
				ProxyHost:     "ip:1111",
				ProxyUsername: "user",
				ProxyPassword: "pass",
			},
			fields: fields{
				Log: log.Real,
			},
			proxyString: validProxySecret,
			wantErr:     false,
		},
		{
			name: "when given a proxy with username/pass containing special chars, should proxy config with username/pass",
			want: stunnelProxyConfig{
				ProxyHost:     "ip:1111",
				ProxyUsername: "user",
				ProxyPassword: "!:pass",
			},
			fields: fields{
				Log: log.Real,
			},
			proxyString: basicAuthWithSpecialChars,
			wantErr:     false,
		},
		{
			name: "when given a proxy with invalid host/port, should return err",
			want: stunnelProxyConfig{},
			fields: fields{
				Log: log.Real,
			},
			proxyString: invalidProxy,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Log: tt.fields.Log,
			}
			settings.Settings.StunnelTCPProxy = tt.proxyString
			got, err := task.generateStunnelProxyConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("Task.generateStunnelProxyConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Task.generateStunnelProxyConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
