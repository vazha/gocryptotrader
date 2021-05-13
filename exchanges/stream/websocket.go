package stream

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vazha/gocryptotrader/config"
	"github.com/vazha/gocryptotrader/log"
)

const (
	defaultJobBuffer = 1000
	// defaultTrafficPeriod defines a period of pause for the traffic monitor,
	// as there are periods with large incoming traffic alerts which requires a
	// timer reset, this limits work on this routine to a more effective rate
	// of check.
	defaultTrafficPeriod = time.Second
)

var errClosedConnection = errors.New("use of closed network connection")

// New initialises the websocket struct
func New() *Websocket {
	return &Websocket{
		Init:              true,
		DataHandler:       make(chan interface{}),
		ToRoutine:         make(chan interface{}, defaultJobBuffer),
		TrafficAlert:      make(chan string),
		ReadMessageErrors: make(chan error),
		Subscribe:         make(chan []ChannelSubscription),
		Unsubscribe:       make(chan []ChannelSubscription),
		Match:             NewMatch(),
	}
}

// Setup sets main variables for websocket connection
func (w *Websocket) Setup(s *WebsocketSetup) error {
	if w == nil {
		return errors.New("websocket is nil")
	}

	if s == nil {
		return errors.New("websocket setup is nil")
	}

	if !w.Init {
		return fmt.Errorf("%s Websocket already initialised",
			s.ExchangeName)
	}

	w.verbose = s.Verbose
	if s.Features == nil {
		return errors.New("websocket features is unset")
	}

	w.features = s.Features

	if w.features.Subscribe && s.Subscriber == nil {
		return errors.New("features have been set yet channel subscriber is not set")
	}
	w.Subscriber = s.Subscriber

	if w.features.Unsubscribe && s.UnSubscriber == nil {
		return errors.New("features have been set yet channel unsubscriber is not set")
	}
	w.Unsubscriber = s.UnSubscriber

	w.GenerateSubs = s.GenerateSubscriptions
	w.GenerateAuthSubs = s.GenerateAuthenticatedSubscriptions

	w.enabled = s.Enabled
	if s.DefaultURL == "" {
		return errors.New("default url is empty")
	}
	w.defaultURL = s.DefaultURL
	if s.RunningURL == "" {
		return errors.New("running URL cannot be nil")
	}
	err := w.SetWebsocketURL(s.RunningURL, false, false)
	if err != nil {
		return err
	}

	if s.RunningURLAuth != "" {
		fmt.Println("RunningURLAuth:", s.RunningURLAuth) // @todo del
		err = w.SetWebsocketURL(s.RunningURLAuth, true, false)
		if err != nil {
			return err
		}
	}

	w.connector = s.Connector
	if s.ExchangeName == "" {
		return errors.New("exchange name unset")
	}
	w.exchangeName = s.ExchangeName

	if s.WebsocketTimeout < time.Second {
		return fmt.Errorf("traffic timeout cannot be less than %s", time.Second)
	}
	fmt.Println("WebsocketTimeout", s.WebsocketTimeout)
	w.trafficTimeout = s.WebsocketTimeout

	w.ShutdownC = make(chan struct{})
	w.Wg = new(sync.WaitGroup)
	w.SetCanUseAuthenticatedEndpoints(s.AuthenticatedWebsocketAPISupport)

	return w.Orderbook.Setup(s.OrderbookBufferLimit,
		s.BufferEnabled,
		s.SortBuffer,
		s.SortBufferByUpdateIDs,
		s.UpdateEntriesByID,
		w.exchangeName,
		w.DataHandler)
}

// SetupNewConnection sets up an auth or unauth streaming connection
func (w *Websocket) SetupNewConnection(c ConnectionSetup) error {
	if w == nil {
		return errors.New("setting up new connection error: websocket is nil")
	}
	if c == (ConnectionSetup{}) {
		return errors.New("setting up new connection error: websocket connection configuration empty")
	}

	if w.exchangeName == "" {
		return errors.New("setting up new connection error: exchange name not set, please call setup first")
	}

	if w.TrafficAlert == nil {
		return errors.New("setting up new connection error: traffic alert is nil, please call setup first")
	}

	if w.ReadMessageErrors == nil {
		return errors.New("setting up new connection error: read message errors is nil, please call setup first")
	}

	connectionURL := w.GetWebsocketURL()
	if c.URL != "" {
		connectionURL = c.URL
	}

	newConn := &WebsocketConnection{
		ExchangeName:      w.exchangeName,
		URL:               connectionURL,
		ProxyURL:          w.GetProxyAddress(),
		Verbose:           w.verbose,
		ResponseMaxLimit:  c.ResponseMaxLimit,
		Traffic:           w.TrafficAlert,
		readMessageErrors: w.ReadMessageErrors,
		ShutdownC:         w.ShutdownC,
		Wg:                w.Wg,
		Match:             w.Match,
		RateLimit:         c.RateLimit,
	}

	if c.Authenticated {
		w.AuthConn = newConn
	} else {
		w.Conn = newConn
	}

	return nil
}

// Connect initiates a websocket connection by using a package defined connection
// function
func (w *Websocket) Connect() error {
	if w.connector == nil {
		return errors.New("websocket connect function not set, cannot continue")
	}
	w.m.Lock()
	defer w.m.Unlock()

	if !w.IsEnabled() {
		return errors.New(WebsocketNotEnabled)
	}
	if w.IsConnecting() {
		return fmt.Errorf("%v Websocket already attempting to connect",
			w.exchangeName)
	}
	if w.IsConnected() {
		return fmt.Errorf("%v Websocket already connected",
			w.exchangeName)
	}

	w.dataMonitor()
	// w.trafficMonitor()
	w.setConnectingStatus(true)

	if !w.IsConnectionMonitorRunning() {
		w.connectionMonitor()
	}

	fmt.Printf("%s, w.connector go, WG: %v\n", w.exchangeName, w.Wg)
	err := w.connector()
	if err != nil {
		w.setConnectingStatus(false)
		return fmt.Errorf("%v Error connecting %s",
			w.exchangeName, err)
	}
	w.trafficMonitor()
	w.setConnectedStatus(true)
	w.setConnectingStatus(false)
	w.setInit(true)

	//if !w.IsConnectionMonitorRunning() {
	//	w.connectionMonitor()
	//}

	return nil
}

// Disable disables the exchange websocket protocol
func (w *Websocket) Disable() error {
	if !w.IsConnected() || !w.IsEnabled() {
		return fmt.Errorf("websocket is already disabled for exchange %s",
			w.exchangeName)
	}

	w.setEnabled(false)
	return nil
}

// Enable enables the exchange websocket protocol
func (w *Websocket) Enable() error {
	if w.IsConnected() || w.IsEnabled() {
		return fmt.Errorf("websocket is already enabled for exchange %s",
			w.exchangeName)
	}

	w.setEnabled(true)
	return w.Connect()
}

// dataMonitor monitors job throughput and logs if there is a back log of data
func (w *Websocket) dataMonitor() {
	if w.IsDataMonitorRunning() {
		return
	}
	w.setDataMonitorRunning(true)
	w.Wg.Add(1)
	fmt.Printf("%s, dataMonitor start. WG: %v\n", w.exchangeName, w.Wg)
	go func() {
		defer func() {
			fmt.Println("dataMonitor finish")
			for {
				// Bleeds data from the websocket connection if needed
				select {
				case <-w.DataHandler:
				default:
					w.setDataMonitorRunning(false)
					w.Wg.Done()
					return
				}
			}
		}()

		for {
			select {
			case <-w.ShutdownC:
				return
			case d := <-w.DataHandler:
				select {
				case w.ToRoutine <- d:
				case <-w.ShutdownC:
					return
				default:
					log.Warnf(log.WebsocketMgr,
						"%s exchange backlog in websocket processing detected",
						w.exchangeName)
					select {
					case w.ToRoutine <- d:
					case <-w.ShutdownC:
						return
					}
				}
			}
		}
	}()
}

// connectionMonitor ensures that the WS keeps connecting
func (w *Websocket) connectionMonitor() {
	if w.IsConnectionMonitorRunning() {
		return
	}
	w.setConnectionMonitorRunning(true)
	fmt.Printf("%s, connectionMonitor Start\n", w.exchangeName)
	go func() {
		defer func() {
			fmt.Println("connectionMonitor Exit - very strange")
		}()
		timer := time.NewTimer(connectionMonitorDelay)

		for {
			if w.verbose {
				log.Debugf(log.WebsocketMgr,
					"%v websocket: running connection monitor cycle\n",
					w.exchangeName)
			}
			if !w.IsEnabled() {
				if w.verbose {
					log.Debugf(log.WebsocketMgr,
						"%v websocket: connectionMonitor - websocket disabled, shutting down\n",
						w.exchangeName)
				}
				if w.IsConnected() {
					err := w.Shutdown()
					if err != nil {
						log.Error(log.WebsocketMgr, err)
					}
				}
				if w.verbose {
					log.Debugf(log.WebsocketMgr,
						"%v websocket: connection monitor exiting\n",
						w.exchangeName)
				}
				timer.Stop()
				w.setConnectionMonitorRunning(false)
				return
			}
			select {
			case err := <-w.ReadMessageErrors:
				if isDisconnectionError(err) {
					w.setInit(false)
					log.Warnf(log.WebsocketMgr,
						"%v websocket has been disconnected. Reason: %v",
						w.exchangeName, err)
					w.setConnectedStatus(false)

				} else {
					// pass off non disconnect errors to datahandler to manage
					w.DataHandler <- err
				}
			case <-timer.C:
				//fmt.Println("connectionMonitor_1", w.IsConnecting(), w.IsConnected())
				if !w.IsConnecting() && !w.IsConnected() {
					//fmt.Println("connectionMonitor_2")
					err := w.Connect()
					if err != nil {
						fmt.Printf("%s, w.Connect error. wg: %v\n", w.exchangeName, w.Wg)
						log.Error(log.WebsocketMgr, err)
					} else {
						fmt.Printf("%s, w.Connect done. wg: %v\n", w.exchangeName, w.Wg)
						err = w.FlushChannels2()
						if err != nil {
							fmt.Printf("%s, w.FlushChannels2 err. wg: %v\n", w.exchangeName, w.Wg)
							log.Error(log.WebsocketMgr, err)
						}
						fmt.Printf("%s, w.FlushChannels2 done. wg: %v\n", w.exchangeName, w.Wg)
					}
				}
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(connectionMonitorDelay)
			}
		}
	}()
}

// Shutdown attempts to shut down a websocket connection and associated routines
// by using a package defined shutdown function
func (w *Websocket) Shutdown() error {
	w.m.Lock()
	defer w.m.Unlock()

	if !w.IsConnected() {
		return fmt.Errorf("%v websocket: cannot shutdown a disconnected websocket",
			w.exchangeName)
	}

	if w.IsConnecting() {
		return fmt.Errorf("%v websocket: cannot shutdown, in the process of reconnection",
			w.exchangeName)
	}

	if w.verbose {
		log.Debugf(log.WebsocketMgr,
			"%v websocket: shutting down websocket\n",
			w.exchangeName)
	}

	defer w.Orderbook.FlushBuffer()
	//fmt.Println("Shutdown_01")
	var connErr, AuthConnErr error
	if w.Conn != nil {
		connErr = w.Conn.Shutdown()
	}
	//fmt.Println("Shutdown_02")
	if w.AuthConn != nil {
		AuthConnErr = w.AuthConn.Shutdown()
	}

	// flush any subscriptions from last connection if needed
	w.subscriptionMutex.Lock()
	w.subscriptions = nil
	w.subscriptionMutex.Unlock()


	fmt.Printf("%s, Shutdown command sent. wg: %v\n", w.exchangeName, w.Wg)
	close(w.ShutdownC)
	fmt.Printf("%s, Shutdown command pre. wg: %v\n", w.exchangeName, w.Wg)
	w.Wg.Wait()
	fmt.Printf("%s, Shutdown command done. wg: %v\n", w.exchangeName, w.Wg)
	w.ShutdownC = make(chan struct{})
	w.setConnectedStatus(false)
	w.setConnectingStatus(false)

	if connErr != nil {
		return connErr
	}

	if AuthConnErr != nil {
		return AuthConnErr
	}

	if w.verbose {
		log.Debugf(log.WebsocketMgr,
			"%v websocket: completed websocket shutdown\n",
			w.exchangeName)
	}
	return nil
}

// FlushChannels flushes channel subscriptions when there is a pair/asset change
func (w *Websocket) FlushChannels() (err error) {
	if !w.IsEnabled() {
		return fmt.Errorf("%s websocket: service not enabled", w.exchangeName)
	}

	if !w.IsConnected() {
		return fmt.Errorf("%s websocket: service not connected", w.exchangeName)
	}

	defer func() {
		if err != nil {
			w.setConnectingStatus(false)
			w.setConnectedStatus(false)
		}
	}()

	if w.features.Subscribe {
		newsubs, err := w.GenerateSubs()
		if err != nil {
			return err
		}

		if w.CanUseAuthenticatedEndpoints() && w.GenerateAuthSubs != nil {
			newAuthSubs, err := w.GenerateAuthSubs()
			if err != nil {
				return err
			}
			newsubs = append(newsubs, newAuthSubs...) // add auth websockets subscriptions if enabled
		}
		//fmt.Println("SubscribeToChannels_1", newsubs)
		//fmt.Println("Exist:", w.subscriptions)
		subs, unsubs := w.GetChannelDifference(newsubs)
		//fmt.Println("SubscribeToChannels_2", subs, unsubs)
		if w.features.Unsubscribe {
			if len(unsubs) != 0 {
				//fmt.Println("UnsubscribeChannels", unsubs)
				err := w.UnsubscribeChannels(unsubs)
				if err != nil {
					return err
				}
			}
		}
		//fmt.Println("SubscribeToChannels", subs)
		if len(subs) < 1 {
			return nil
		}

		return w.SubscribeToChannels(subs)
	} else if w.features.FullPayloadSubscribe {
		// FullPayloadSubscribe means that the endpoint requires all
		// subscriptions to be sent via the websocket connection e.g. if you are
		// subscribed to ticker and orderbook but require trades as well, you
		// would need to send ticker, orderbook and trades channel subscription
		// messages.
		newsubs, err := w.GenerateSubs()
		if err != nil {
			return err
		}

		if w.CanUseAuthenticatedEndpoints() && w.GenerateAuthSubs != nil {
			newAuthSubs, err := w.GenerateAuthSubs()
			if err != nil {
				return err
			}
			newsubs = append(newsubs, newAuthSubs...) // add auth websockets subscriptions if enabled
		}

		if len(newsubs) != 0 {
			// Purge subscription list as there will be conflicts
			w.subscriptionMutex.Lock()
			w.subscriptions = nil
			w.subscriptionMutex.Unlock()
			return w.SubscribeToChannels(newsubs)
		}
		return nil
	}

	return w.Shutdown()
}

// FlushChannels2 flushes channel subscriptions when there is a pair/asset change
func (w *Websocket) FlushChannels2() (err error) {
	if !w.IsEnabled() {
		return fmt.Errorf("%s websocket: service not enabled", w.exchangeName)
	}

	if !w.IsConnected() {
		return fmt.Errorf("%s websocket: service not connected", w.exchangeName)
	}

	defer func() {
		if err != nil {
			w.setConnectingStatus(false)
			w.setConnectedStatus(false)
		}
	}()

	if w.features.Subscribe {
		newsubs, err := w.GenerateSubs()
		if err != nil {
			return err
		}

		if w.CanUseAuthenticatedEndpoints() && w.GenerateAuthSubs != nil {
			newAuthSubs, err := w.GenerateAuthSubs()
			if err != nil {
				return err
			}
			newsubs = append(newsubs, newAuthSubs...) // add auth websockets subscriptions if enabled
		}

		if len(newsubs) < 1 {
			return nil
		}

		return w.SubscribeToChannels2(newsubs)
	} else if w.features.FullPayloadSubscribe {
		// FullPayloadSubscribe means that the endpoint requires all
		// subscriptions to be sent via the websocket connection e.g. if you are
		// subscribed to ticker and orderbook but require trades as well, you
		// would need to send ticker, orderbook and trades channel subscription
		// messages.
		newsubs, err := w.GenerateSubs()
		if err != nil {
			return err
		}

		if w.CanUseAuthenticatedEndpoints() && w.GenerateAuthSubs != nil {
			newAuthSubs, err := w.GenerateAuthSubs()
			if err != nil {
				return err
			}
			newsubs = append(newsubs, newAuthSubs...) // add auth websockets subscriptions if enabled
		}

		if len(newsubs) != 0 {
			// Purge subscription list as there will be conflicts
			w.subscriptionMutex.Lock()
			w.subscriptions = nil
			w.subscriptionMutex.Unlock()
			return w.SubscribeToChannels(newsubs)
		}
		return nil
	}

	return w.Shutdown()
}

// trafficMonitor uses a timer of WebsocketTrafficLimitTime and once it expires,
// it will reconnect if the TrafficAlert channel has not received any data. The
// trafficTimer will reset on each traffic alert
func (w *Websocket) trafficMonitor() {
	if w.IsTrafficMonitorRunning() {
		return
	}
	w.setTrafficMonitorRunning(true)
	w.Wg.Add(1)
	fmt.Printf("%s, trafficMonitor start. WG: %v\n", w.exchangeName, w.Wg)

	w.trafficTimeout = time.Second * 45 // todo delete
	AuthTrafficTimeout := time.Second * 90 // todo delete

	go func() {
		defer func() {
			fmt.Printf("%s, trafficMonitor finish wg: %v\n", w.exchangeName, w.Wg)
		}()
		var trafficTimer = time.NewTimer(w.trafficTimeout)
		var trafficAuthTimer = time.NewTimer(w.trafficTimeout)
		if !w.CanUseAuthenticatedEndpoints() {
			trafficAuthTimer.Stop()
		}

		if w.CanUseAuthenticatedEndpoints() && w.AuthConn == nil{
			trafficAuthTimer.Stop()
		}

		for {
			select {
			case <-w.ShutdownC:
				if w.verbose {
					log.Debugf(log.WebsocketMgr,
						"%v websocket: trafficMonitor shutdown message received\n",
						w.exchangeName)
				}
				trafficTimer.Stop()
				w.setTrafficMonitorRunning(false)
				w.Wg.Done()
				return
			case t := <-w.TrafficAlert:
				//fmt.Println("TrafficAlert", t)
				switch {
				case w.AuthConn != nil && w.AuthConn.GetURL() == t:
					if t == "wss://stream.crypto.com/v2/user" {
						// fmt.Println("TrafficAlert Auth", t)
					}

					if !trafficAuthTimer.Stop() {
						select {
						case <-trafficAuthTimer.C:
						default:
						}
					}
					w.setConnectedStatus(true)
					trafficAuthTimer.Reset(AuthTrafficTimeout)
				case w.Conn != nil && w.Conn.GetURL() == t:
					if t == "wss://stream.crypto.com/v2/market" {
						//fmt.Println("TrafficAlert NON Auth", t)
					}

					if !trafficTimer.Stop() {
						select {
						case <-trafficTimer.C:
						default:
						}
					}
					w.setConnectedStatus(true)
					trafficTimer.Reset(w.trafficTimeout)
				default:
					log.Warnf(log.WebsocketMgr, "Unhandled traffic alert for url: %s", t)
				}
			case <-trafficTimer.C: // Falls through when timer runs out
				fmt.Printf("%s, trafficTimer.C !!!. wg: %v\n", w.exchangeName, w.Wg)
				if w.verbose {
					log.Warnf(log.WebsocketMgr,
						"%v websocket: has not received a traffic alert in %v. Reconnecting",
						w.exchangeName,
						w.trafficTimeout)
				}
				trafficTimer.Stop()
				w.Wg.Done()
				//fmt.Println("trafficTimer_2", w.IsConnecting() , w.IsConnected() )
				if !w.IsConnecting() && w.IsConnected() {
					fmt.Printf("%s, call Shutdown. wg: %v\n", w.exchangeName, w.Wg)
					err := w.Shutdown()
					if err != nil {
						log.Errorf(log.WebsocketMgr,
							"%v websocket: trafficMonitor shutdown err: %s",
							w.exchangeName, err)
					}
				}
				w.setTrafficMonitorRunning(false)
				return
			case <-trafficAuthTimer.C: // Falls through when auth timer runs out
				fmt.Printf("%s, trafficAuthTimer.C !!!. wg: %v\n", w.exchangeName, w.Wg)
				//fmt.Println("trafficAuthTimer:", w.Conn.GetURL())
				if w.verbose {
					log.Warnf(log.WebsocketMgr,
						"%v auth websocket: has not received a traffic alert in %v. Reconnecting",
						w.exchangeName,
						AuthTrafficTimeout)
				}
				trafficAuthTimer.Stop()
				w.Wg.Done()
				if !w.IsConnecting() && w.IsConnected() {
					fmt.Printf("%s, call Auth Shutdown. wg: %v\n", w.exchangeName, w.Wg)
					err := w.Shutdown()
					if err != nil {
						log.Errorf(log.WebsocketMgr,
							"%v auth websocket: trafficMonitor shutdown err: %s",
							w.exchangeName, err)
					}
				}
				w.setTrafficMonitorRunning(false)
				return
			}
		}
	}()
}

func (w *Websocket) setConnectedStatus(b bool) {
	w.connectionMutex.Lock()
	w.connected = b
	w.connectionMutex.Unlock()
}

// IsConnected returns status of connection
func (w *Websocket) IsConnected() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.connected
}

func (w *Websocket) setConnectingStatus(b bool) {
	w.connectionMutex.Lock()
	w.connecting = b
	w.connectionMutex.Unlock()
}

// IsConnecting returns status of connecting
func (w *Websocket) IsConnecting() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.connecting
}

func (w *Websocket) setEnabled(b bool) {
	w.connectionMutex.Lock()
	w.enabled = b
	w.connectionMutex.Unlock()
}

// IsEnabled returns status of enabled
func (w *Websocket) IsEnabled() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.enabled
}

func (w *Websocket) setInit(b bool) {
	w.connectionMutex.Lock()
	w.Init = b
	w.connectionMutex.Unlock()
}

// IsInit returns status of init
func (w *Websocket) IsInit() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.Init
}

func (w *Websocket) setTrafficMonitorRunning(b bool) {
	w.connectionMutex.Lock()
	w.trafficMonitorRunning = b
	w.connectionMutex.Unlock()
}

// IsTrafficMonitorRunning returns status of the traffic monitor
func (w *Websocket) IsTrafficMonitorRunning() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.trafficMonitorRunning
}

func (w *Websocket) setConnectionMonitorRunning(b bool) {
	w.connectionMutex.Lock()
	w.connectionMonitorRunning = b
	w.connectionMutex.Unlock()
}

// IsConnectionMonitorRunning returns status of connection monitor
func (w *Websocket) IsConnectionMonitorRunning() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.connectionMonitorRunning
}

func (w *Websocket) setDataMonitorRunning(b bool) {
	w.connectionMutex.Lock()
	w.dataMonitorRunning = b
	w.connectionMutex.Unlock()
}

// IsDataMonitorRunning returns status of data monitor
func (w *Websocket) IsDataMonitorRunning() bool {
	w.connectionMutex.RLock()
	defer w.connectionMutex.RUnlock()
	return w.dataMonitorRunning
}

// CanUseAuthenticatedWebsocketForWrapper Handles a common check to
// verify whether a wrapper can use an authenticated websocket endpoint
func (w *Websocket) CanUseAuthenticatedWebsocketForWrapper() bool {
	if w.IsConnected() && w.CanUseAuthenticatedEndpoints() {
		return true
	} else if w.IsConnected() && !w.CanUseAuthenticatedEndpoints() {
		log.Infof(log.WebsocketMgr,
			WebsocketNotAuthenticatedUsingRest,
			w.exchangeName)
	}
	return false
}

// SetWebsocketURL sets websocket URL and can refresh underlying connections
func (w *Websocket) SetWebsocketURL(url string, auth, reconnect bool) error {
	defaultVals := url == "" || url == config.WebsocketURLNonDefaultMessage
	if auth {
		if defaultVals {
			url = w.defaultURLAuth
		}

		err := checkWebsocketURL(url)
		if err != nil {
			return err
		}
		w.runningURLAuth = url

		if w.verbose {
			log.Debugf(log.WebsocketMgr,
				"%s websocket: setting authenticated websocket URL: %s\n",
				w.exchangeName,
				url)
		}

		if w.AuthConn != nil {
			w.AuthConn.SetURL(url)
		}
	} else {
		if defaultVals {
			url = w.defaultURL
		}
		err := checkWebsocketURL(url)
		if err != nil {
			return err
		}
		w.runningURL = url

		if w.verbose {
			log.Debugf(log.WebsocketMgr,
				"%s websocket: setting unauthenticated websocket URL: %s\n",
				w.exchangeName,
				url)
		}

		if w.Conn != nil {
			w.Conn.SetURL(url)
		}
	}

	if w.IsConnected() && reconnect {
		log.Debugf(log.WebsocketMgr,
			"%s websocket: flushing websocket connection to %s\n",
			w.exchangeName,
			url)
		return w.Shutdown()
	}
	return nil
}

// GetWebsocketURL returns the running websocket URL
func (w *Websocket) GetWebsocketURL() string {
	return w.runningURL
}

// SetProxyAddress sets websocket proxy address
func (w *Websocket) SetProxyAddress(proxyAddr string) error {
	if proxyAddr != "" {
		_, err := url.ParseRequestURI(proxyAddr)
		if err != nil {
			return fmt.Errorf("%v websocket: cannot set proxy address error '%v'",
				w.exchangeName,
				err)
		}

		if w.proxyAddr == proxyAddr {
			return fmt.Errorf("%v websocket: cannot set proxy address to the same address '%v'",
				w.exchangeName,
				w.proxyAddr)
		}

		log.Debugf(log.ExchangeSys,
			"%s websocket: setting websocket proxy: %s\n",
			w.exchangeName,
			proxyAddr)
	} else {
		log.Debugf(log.ExchangeSys,
			"%s websocket: removing websocket proxy\n",
			w.exchangeName)
	}

	if w.Conn != nil {
		w.Conn.SetProxy(proxyAddr)
	}
	if w.AuthConn != nil {
		w.AuthConn.SetProxy(proxyAddr)
	}

	w.proxyAddr = proxyAddr
	if w.IsInit() && w.IsEnabled() {
		if w.IsConnected() {
			err := w.Shutdown()
			if err != nil {
				return err
			}
		}

		err := w.Connect()
		if err != nil {
			return err
		}
		return w.FlushChannels()
	}
	return nil
}

// GetProxyAddress returns the current websocket proxy
func (w *Websocket) GetProxyAddress() string {
	return w.proxyAddr
}

// GetName returns exchange name
func (w *Websocket) GetName() string {
	return w.exchangeName
}

// GetChannelDifference finds the difference between the subscribed channels
// and the new subscription list when pairs are disabled or enabled.
func (w *Websocket) GetChannelDifference(genSubs []ChannelSubscription) (sub, unsub []ChannelSubscription) {
	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()

oldsubs:
	for x := range w.subscriptions {
		for y := range genSubs {
			if w.subscriptions[x].Equal(&genSubs[y]) {
				continue oldsubs
			}
		}
		unsub = append(unsub, w.subscriptions[x])
	}

newsubs:
	for x := range genSubs {
		for y := range w.subscriptions {
			if genSubs[x].Equal(&w.subscriptions[y]) {
				continue newsubs
			}
		}
		sub = append(sub, genSubs[x])
	}
	return
}

// UnsubscribeChannels unsubscribes from a websocket channel
func (w *Websocket) UnsubscribeChannels(channels []ChannelSubscription) error {
	if len(channels) == 0 {
		return fmt.Errorf("%s websocket: channels not populated cannot remove",
			w.exchangeName)
	}
	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()

channels:
	for x := range channels {
		for y := range w.subscriptions {
			if channels[x].Equal(&w.subscriptions[y]) {
				continue channels
			}
		}
		return fmt.Errorf("%s websocket: subscription not found in list: %+v",
			w.exchangeName,
			channels[x])
	}
	return w.Unsubscriber(channels)
}

// ResubscribeToChannel resubscribes to channel
func (w *Websocket) ResubscribeToChannel(subscribedChannel *ChannelSubscription) error {
	err := w.UnsubscribeChannels([]ChannelSubscription{*subscribedChannel})
	if err != nil {
		return err
	}
	return w.SubscribeToChannels([]ChannelSubscription{*subscribedChannel})
}

// SubscribeToChannels appends supplied channels to channelsToSubscribe
func (w *Websocket) SubscribeToChannels(channels []ChannelSubscription) error {
	if len(channels) == 0 {
		return fmt.Errorf("%s websocket: cannot subscribe no channels supplied",
			w.exchangeName)
	}

	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()
	for x := range channels {
		for y := range w.subscriptions {
			if channels[x].Equal(&w.subscriptions[y]) {
				return fmt.Errorf("%s websocket: %v already subscribed",
					w.exchangeName,
					channels[x])
			}
		}
	}
	return w.Subscriber(channels)
}

// SubscribeToChannels2 appends supplied channels to channelsToSubscribe
func (w *Websocket) SubscribeToChannels2(channels []ChannelSubscription) error {
	if len(channels) == 0 {
		return fmt.Errorf("%s websocket: cannot subscribe no channels supplied",
			w.exchangeName)
	}

	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()

	return w.Subscriber(channels)
}

// AddSuccessfulSubscriptions adds subscriptions to the subscription lists that
// has been successfully subscribed
func (w *Websocket) AddSuccessfulSubscriptions(channels ...ChannelSubscription) {
	w.subscriptions = append(w.subscriptions, channels...)
}

// RemoveSuccessfulUnsubscriptions removes subscriptions from the subscription
// list that has been successfulling unsubscribed
func (w *Websocket) RemoveSuccessfulUnsubscriptions(channels ...ChannelSubscription) {
	for x := range channels {
		for y := range w.subscriptions {
			if channels[x].Equal(&w.subscriptions[y]) {
				w.subscriptions[y] = w.subscriptions[len(w.subscriptions)-1]
				w.subscriptions[len(w.subscriptions)-1] = ChannelSubscription{}
				w.subscriptions = w.subscriptions[:len(w.subscriptions)-1]
				break
			}
		}
	}
}

// Equal two WebsocketChannelSubscription to determine equality
func (w *ChannelSubscription) Equal(s *ChannelSubscription) bool {
	return strings.EqualFold(w.Channel, s.Channel) &&
		w.Currency.Equal(s.Currency)
}

// GetSubscriptions returns a copied list of subscriptions
// subscriptions is a private member and cannot be manipulated
func (w *Websocket) GetSubscriptions() []ChannelSubscription {
	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()
	return append(w.subscriptions[:0:0], w.subscriptions...)
}

// SetCanUseAuthenticatedEndpoints sets canUseAuthenticatedEndpoints val in
// a thread safe manner
func (w *Websocket) SetCanUseAuthenticatedEndpoints(val bool) {
	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()
	w.canUseAuthenticatedEndpoints = val
}

// CanUseAuthenticatedEndpoints gets canUseAuthenticatedEndpoints val in
// a thread safe manner
func (w *Websocket) CanUseAuthenticatedEndpoints() bool {
	w.subscriptionMutex.Lock()
	defer w.subscriptionMutex.Unlock()
	return w.canUseAuthenticatedEndpoints
}

// isDisconnectionError Determines if the error sent over chan ReadMessageErrors is a disconnection error
func isDisconnectionError(err error) bool {
	if websocket.IsUnexpectedCloseError(err) {
		return true
	}
	if _, ok := err.(*net.OpError); ok {
		return !errors.Is(err, errClosedConnection)
	}
	return false
}

// checkWebsocketURL checks for a valid websocket url
func checkWebsocketURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("cannot set invalid websocket URL %s", s)
	}
	return nil
}
