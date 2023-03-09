package stream

//func addStreamCommand(a *app) *cobra.Command {
//	var socketPort uint32
//	var logLevel string
//
//	cmd := &cobra.Command{
//		Use:   "stream",
//		Short: "Start a websocket server to pipe events to a convoy cli instance",
//		RunE: func(cmd *cobra.Command, args []string) error {
//			c, err := config.Get()
//			if err != nil {
//				a.logger.WithError(err).Fatal("failed to initialize realm chain")
//				return err
//			}
//
//			endpointRepo := postgres.NewEndpointRepo(a.db)
//			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.db)
//			sourceRepo := postgres.NewSourceRepo(a.db)
//			subRepo := postgres.NewSubscriptionRepo(a.db)
//			deviceRepo := postgres.NewDeviceRepo(a.db)
//			projectRepo := postgres.NewProjectRepo(a.db)
//			apiKeyRepo := postgres.NewAPIKeyRepo(a.db)
//			userRepo := postgres.NewUserRepo(a.db)
//			orgMemberRepo := postgres.NewOrgMemberRepo(a.db)
//
//			// enable only the native auth realm
//			authCfg := &config.AuthConfiguration{
//				Native: config.NativeRealmOptions{Enabled: true},
//			}
//
//			err = realm_chain.Init(authCfg, apiKeyRepo, userRepo, nil)
//			if err != nil {
//				a.logger.WithError(err).Fatal("failed to initialize realm chain")
//				return err
//			}
//
//			r := &socket.Repo{
//				OrgMemberRepository: orgMemberRepo,
//				ProjectRepo:         projectRepo,
//				EndpointRepo:        endpointRepo,
//				DeviceRepo:          deviceRepo,
//				SubscriptionRepo:    subRepo,
//				SourceRepo:          sourceRepo,
//				EventDeliveryRepo:   eventDeliveryRepo,
//			}
//
//			h := socket.NewHub()
//			h.Start()
//
//			lo := a.logger.(*log.Logger)
//			lo.SetPrefix("socket server")
//
//			lvl, err := log.ParseLevel(c.Logger.Level)
//			if err != nil {
//				return err
//			}
//			lo.SetLevel(lvl)
//
//			m := convoyMiddleware.NewMiddleware(&convoyMiddleware.CreateMiddleware{
//				UserRepo:     userRepo,
//				EndpointRepo: endpointRepo,
//				ProjectRepo:  projectRepo,
//				Cache:        a.cache,
//				Logger:       lo,
//			})
//
//			handler := socket.BuildRoutes(h, r, m)
//
//			if c.Server.HTTP.SocketPort != 0 {
//				socketPort = c.Server.HTTP.SocketPort
//			}
//
//			srv := server.NewServer(socketPort, func() {
//				h.Stop()
//			})
//
//			srv.SetHandler(handler)
//
//			a.logger.Infof("Stream server running on port %v", socketPort)
//			srv.Listen()
//
//			return nil
//		},
//	}
//
//	cmd.Flags().Uint32Var(&socketPort, "socket-port", 5008, "Socket port")
//	cmd.Flags().StringVar(&logLevel, "log-level", "error", "stream log level")
//	return cmd
//}
