package api_rest

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/bettercap/bettercap/session"

	"github.com/gorilla/mux"
)

type CommandRequest struct {
	Command string `json:"cmd"`
}

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"msg"`
}

func (mod *RestAPI) setAuthFailed(w http.ResponseWriter, r *http.Request) {
	mod.Warning("Unauthorized authentication attempt from %s to %s", r.RemoteAddr, r.URL.String())

	w.Header().Set("WWW-Authenticate", `Basic realm="auth"`)
	w.WriteHeader(401)
	w.Write([]byte("Unauthorized"))
}

func (mod *RestAPI) toJSON(w http.ResponseWriter, o interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(o); err != nil {
		mod.Error("error while encoding object to JSON: %v", err)
	}
}

func (mod *RestAPI) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Add("X-Frame-Options", "DENY")
	w.Header().Add("X-Content-Type-Options", "nosniff")
	w.Header().Add("X-XSS-Protection", "1; mode=block")
	w.Header().Add("Referrer-Policy", "same-origin")

	w.Header().Set("Access-Control-Allow-Origin", mod.allowOrigin)
	w.Header().Add("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
}

func (mod *RestAPI) checkAuth(r *http.Request) bool {
	if mod.username != "" && mod.password != "" {
		user, pass, _ := r.BasicAuth()
		// timing attack my ass
		if subtle.ConstantTimeCompare([]byte(user), []byte(mod.username)) != 1 {
			return false
		} else if subtle.ConstantTimeCompare([]byte(pass), []byte(mod.password)) != 1 {
			return false
		}
	}
	return true
}

func (mod *RestAPI) showSession(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I)
}

func (mod *RestAPI) showBLE(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	mac := strings.ToLower(params["mac"])

	if mac == "" {
		mod.toJSON(w, session.I.BLE)
	} else if dev, found := session.I.BLE.Get(mac); found {
		mod.toJSON(w, dev)
	} else {
		http.Error(w, "Not Found", 404)
	}
}

func (mod *RestAPI) showHID(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	mac := strings.ToLower(params["mac"])

	if mac == "" {
		mod.toJSON(w, session.I.HID)
	} else if dev, found := session.I.HID.Get(mac); found {
		mod.toJSON(w, dev)
	} else {
		http.Error(w, "Not Found", 404)
	}
}

func (mod *RestAPI) showEnv(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.Env)
}

func (mod *RestAPI) showGateway(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.Gateway)
}

func (mod *RestAPI) showInterface(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.Interface)
}

func (mod *RestAPI) showModules(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.Modules)
}

func (mod *RestAPI) showLAN(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	mac := strings.ToLower(params["mac"])

	if mac == "" {
		mod.toJSON(w, session.I.Lan)
	} else if host, found := session.I.Lan.Get(mac); found {
		mod.toJSON(w, host)
	} else {
		http.Error(w, "Not Found", 404)
	}
}

func (mod *RestAPI) showOptions(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.Options)
}

func (mod *RestAPI) showPackets(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.Queue)
}

func (mod *RestAPI) showStartedAt(w http.ResponseWriter, r *http.Request) {
	mod.toJSON(w, session.I.StartedAt)
}

func (mod *RestAPI) showWiFi(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	mac := strings.ToLower(params["mac"])

	if mac == "" {
		mod.toJSON(w, session.I.WiFi)
	} else if station, found := session.I.WiFi.Get(mac); found {
		mod.toJSON(w, station)
	} else if client, found := session.I.WiFi.GetClient(mac); found {
		mod.toJSON(w, client)
	} else {
		http.Error(w, "Not Found", 404)
	}
}

func (mod *RestAPI) runSessionCommand(w http.ResponseWriter, r *http.Request) {
	var err error
	var cmd CommandRequest

	if r.Body == nil {
		http.Error(w, "Bad Request", 400)
	} else if err = json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Bad Request", 400)
	} else if err = session.I.Run(cmd.Command); err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		mod.toJSON(w, APIResponse{Success: true})
	}
}

func (mod *RestAPI) showEvents(w http.ResponseWriter, r *http.Request) {
	var err error

	if mod.useWebsocket {
		mod.startStreamingEvents(w, r)
	} else {
		events := session.I.Events.Sorted()
		nevents := len(events)
		nmax := nevents
		n := nmax

		q := r.URL.Query()
		vals := q["n"]
		if len(vals) > 0 {
			n, err = strconv.Atoi(q["n"][0])
			if err == nil {
				if n > nmax {
					n = nmax
				}
			} else {
				n = nmax
			}
		}

		mod.toJSON(w, events[nevents-n:])
	}
}

func (mod *RestAPI) clearEvents(w http.ResponseWriter, r *http.Request) {
	session.I.Events.Clear()
}

func (mod *RestAPI) corsRoute(w http.ResponseWriter, r *http.Request) {
	mod.setSecurityHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func (mod *RestAPI) sessionRoute(w http.ResponseWriter, r *http.Request) {
	mod.setSecurityHeaders(w)

	if !mod.checkAuth(r) {
		mod.setAuthFailed(w, r)
		return
	} else if r.Method == "POST" {
		mod.runSessionCommand(w, r)
		return
	} else if r.Method != "GET" {
		http.Error(w, "Bad Request", 400)
		return
	}

	session.I.Lock()
	defer session.I.Unlock()

	path := r.URL.String()
	switch {
	case path == "/api/session":
		mod.showSession(w, r)

	case path == "/api/session/env":
		mod.showEnv(w, r)

	case path == "/api/session/gateway":
		mod.showGateway(w, r)

	case path == "/api/session/interface":
		mod.showInterface(w, r)

	case strings.HasPrefix(path, "/api/session/modules"):
		mod.showModules(w, r)

	case strings.HasPrefix(path, "/api/session/lan"):
		mod.showLAN(w, r)

	case path == "/api/session/options":
		mod.showOptions(w, r)

	case path == "/api/session/packets":
		mod.showPackets(w, r)

	case path == "/api/session/started-at":
		mod.showStartedAt(w, r)

	case strings.HasPrefix(path, "/api/session/ble"):
		mod.showBLE(w, r)

	case strings.HasPrefix(path, "/api/session/hid"):
		mod.showHID(w, r)

	case strings.HasPrefix(path, "/api/session/wifi"):
		mod.showWiFi(w, r)

	default:
		http.Error(w, "Not Found", 404)
	}
}

func (mod *RestAPI) eventsRoute(w http.ResponseWriter, r *http.Request) {
	mod.setSecurityHeaders(w)

	if !mod.checkAuth(r) {
		mod.setAuthFailed(w, r)
		return
	}

	if r.Method == "GET" {
		mod.showEvents(w, r)
	} else if r.Method == "DELETE" {
		mod.clearEvents(w, r)
	} else {
		http.Error(w, "Bad Request", 400)
	}
}
