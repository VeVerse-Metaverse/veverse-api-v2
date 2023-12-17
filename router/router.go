package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/swagger"
	gf "github.com/shareed2k/goth_fiber"
	"os"
	"veverse-api/handler"
	"veverse-api/middleware"
)

// SetupRoutes setup router api
func SetupRoutes(app *fiber.App) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).Send([]byte{})
	})

	app.Get("/swagger/*", swagger.New(swagger.Config{
		DocExpansion:         "list",
		PersistAuthorization: true,
	}))

	app.Get("/robots.txt", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).SendString("User-agent: *\nDisallow: /")
	})
	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Get("/ads.txt", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	app.Get("/app-ads.txt", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	api := app.Group("/v2", logger.New())

	//region healthcheck
	//api.Get("/health/check", middleware.ProtectedPublicApi(), handler.HealthCheck)
	// end healthcheck

	//region Auth
	auth := api.Group("/auth")
	auth.Post("/login", handler.Login)
	auth.Post("/login/web3", handler.LoginWeb3)
	auth.Post("/restore", handler.SendRecoveryLink)
	auth.Post("/restore/password", handler.RestorePassword)
	auth.Get("/restore/password/check", handler.CheckRestoreToken)
	//endregion

	//region OAuth
	oauthHelper := api.Group("/oauth-helper")
	oauthHelper.Post("/:provider", handler.OAuthHelperCallback)

	oauth := api.Group("/oauth")
	oauth.Get("/:provider/login", gf.BeginAuthHandler)
	oauth.Get("/:provider/callback", handler.OAuthCallback)
	oauth.Get("/:provider/logout", handler.OAuthLogout)
	//endregion

	//region Entity
	entity := api.Group("/entities")
	entity.Get("/:id", middleware.ProtectedJwt(), handler.GetEntity)
	entity.Delete("/:id", middleware.ProtectedJwt(), handler.DeleteEntity)
	entity.Post("/:id/views", middleware.ProtectedJwt(), handler.IncrementEntityView)
	entity.Get("/:id/files", middleware.ProtectedJwt(), handler.IndexFiles)
	entity.Put("/:id/files/upload", middleware.ProtectedJwt(), handler.UploadFile)
	entity.Put("/:id/files/link", middleware.ProtectedJwt(), handler.LinkFile)
	entity.Delete("/files/:id", middleware.ProtectedJwt(), handler.DeleteFile)
	entity.Get("/:id/properties", middleware.ProtectedJwt(), handler.GetProperties)
	entity.Post("/:id/properties", middleware.ProtectedJwt(), handler.AddProperties)
	entity.Get("/:id/access", middleware.ProtectedJwt(), handler.GetEntityAccess)
	entity.Patch("/:id/access", middleware.ProtectedJwt(), handler.UpdateEntityAccess)
	entity.Patch("/:id/public", middleware.ProtectedJwt(), handler.UpdateEntityPublic)
	entity.Get("/:id/tags", middleware.ProtectedJwt(), handler.GetTags)
	entity.Get("/:id/comments", middleware.ProtectedJwt(), handler.GetComments)
	entity.Get("/:id/ratings", middleware.ProtectedJwt(), handler.GetRatings)
	entity.Put("/:id/like", middleware.ProtectedJwt(), handler.LikeEntity)
	entity.Put("/:id/dislike", middleware.ProtectedJwt(), handler.DislikeEntity)
	entity.Put("/:id/unlike", middleware.ProtectedJwt(), handler.UnlikeEntity)
	//endregion

	//region Files
	file := api.Group("/files")
	file.Get("/upload", middleware.ProtectedJwt(), handler.GetFileUploadLink)
	file.Get("/download", middleware.ProtectedJwt(), handler.GetFileDownloadLink)
	file.Get("/download-pre-signed", middleware.ProtectedJwt(), handler.GetFilePreSignedDownloadLink)
	file.Get("/download-pre-signed-url", middleware.ProtectedJwt(), handler.GetFilePreSignedDownloadLinkByURL)
	//endregion

	//region World
	space := api.Group("/spaces")                                                         // Deprecated
	space.Get("", middleware.ProtectedJwt(), handler.IndexWorlds)                         // Deprecated
	space.Get("/:id", middleware.ProtectedJwt(), handler.GetWorld)                        // Deprecated
	space.Get("/:id/placeables", middleware.ProtectedJwt(), handler.IndexWorldPlaceables) // Deprecated
	space.Post("", middleware.ProtectedJwt(), handler.CreateWorld)                        // Deprecated

	world := api.Group("/worlds")
	world.Get("", middleware.ProtectedJwt(), handler.IndexWorlds)
	world.Post("/v2", middleware.ProtectedJwt(), handler.IndexWorldsV2)
	world.Post("/:id/v2", middleware.ProtectedJwt(), handler.GetWorldV2)
	world.Get("/:id", middleware.ProtectedJwt(), handler.GetWorld)
	world.Get("/:id/objects", middleware.ProtectedJwt(), handler.IndexWorldPlaceables)
	world.Post("/:id/objects", middleware.ProtectedJwt(), handler.CreateWorldPlaceable)
	world.Post("", middleware.ProtectedJwt(), handler.CreateWorld)
	world.Patch("/:id", middleware.ProtectedJwt(), handler.UpdateWorld)
	world.Delete("/:id", middleware.ProtectedJwt(), handler.DeleteWorld)
	//endregion

	//region Object
	object := api.Group("/objects")
	object.Get("", middleware.ProtectedJwt(), handler.IndexObjects)
	object.Get("/:id", middleware.ProtectedJwt(), handler.GetObject)
	//endregion

	// region Art Objects
	artObject := api.Group("/art-objects")
	artObject.Get("", middleware.ProtectedJwt(), handler.IndexArtObjects)
	artObject.Get("/:id", middleware.ProtectedJwt(), handler.GetArtObject)

	//region Portal
	portal := api.Group("/portals")
	portal.Get("", middleware.ProtectedJwt(), handler.IndexPortals)
	portal.Get("/:id", middleware.ProtectedJwt(), handler.GetPortal)
	portal.Post("", middleware.ProtectedJwt(), handler.CreatePortal)
	portal.Patch("/:id", middleware.ProtectedJwt(), handler.UpdatePortal)
	//endregion

	//region Object Classes
	placeableClass := api.Group("/placeable_classes")                                                // Deprecated
	placeableClass.Get("", middleware.ProtectedJwt(), handler.IndexObjectClasses)                    // Deprecated
	placeableClass.Get("/categories", middleware.ProtectedJwt(), handler.IndexObjectClassCategories) // Deprecated
	objectClass := api.Group("/object-classes")
	objectClass.Get("", middleware.ProtectedJwt(), handler.IndexObjectClasses)
	objectClass.Get("/categories", middleware.ProtectedJwt(), handler.IndexObjectClassCategories)
	//endregion

	//region Asset Packages
	metaverse := api.Group("/metaverses")                                // Deprecated
	metaverse.Get("", middleware.ProtectedJwt(), handler.IndexPackages)  // Deprecated
	metaverse.Get("/:id", middleware.ProtectedJwt(), handler.GetPackage) // Deprecated
	packages := api.Group("/packages")
	packages.Get("", middleware.ProtectedJwt(), handler.IndexPackages)
	packages.Get("/:id", middleware.ProtectedJwt(), handler.GetPackage)
	packages.Post("", middleware.ProtectedJwt(), handler.CreatePackage)
	packages.Patch("/:id", middleware.ProtectedJwt(), handler.UpdatePackage)
	packages.Get("/:id/maps", middleware.ProtectedJwt(), handler.IndexPackageMaps)
	//packages.Get("/:id/worlds", middleware.ProtectedJwt(), handler.IndexPackageWorlds)
	//endregion

	//region User
	user := api.Group("/users")
	user.Get("/manager", middleware.ProtectedJwt(), handler.IndexUsersForManager)
	user.Get("/nonce", handler.GetNonce)
	user.Get("/me", middleware.ProtectedJwt(), handler.GetMe)                                  // Get requester metadata
	user.Put("/me/name", middleware.ProtectedJwt(), handler.SetName)                           // Set requester name
	user.Get("", middleware.ProtectedJwt(), handler.IndexUsers)                                // Index users
	user.Get("/:id", middleware.ProtectedJwt(), handler.GetUser)                               // Get single user
	user.Get("/:id/followers", middleware.ProtectedJwt(), handler.IndexFollowers)              // Get user followers
	user.Get("/:id/leaders", middleware.ProtectedJwt(), handler.IndexLeaders)                  // Get user leaders
	user.Get("/:id/friends", middleware.ProtectedJwt(), handler.IndexFriends)                  // Get user friends
	user.Get("/:followerId/follows/:leaderId", middleware.ProtectedJwt(), handler.IsFollowing) // Check if user follows another user
	user.Put("/:id/follow", middleware.ProtectedJwt(), handler.Follow)
	user.Delete("/:id/follow", middleware.ProtectedJwt(), handler.Unfollow)
	user.Get("/:id/avatars", middleware.ProtectedJwt(), handler.IndexUserAvatars)
	user.Get("/:id/personas", middleware.ProtectedJwt(), handler.IndexUserPersonas)
	user.Get("/personas/:id", middleware.ProtectedJwt(), handler.GetUserPersona)
	user.Get("/address/:ethAddr", middleware.ProtectedJwt(), handler.GetUserByEthAddress)

	//endregion

	//region Signup
	signup := api.Group("/signup")
	signup.Post("", middleware.ProtectedApi(), handler.Signup)
	//endregion

	//region Events
	events := api.Group("/events")
	events.Post("/checkout/session", middleware.ProtectedJwt(), handler.CreateSessionForCheckout)
	events.Post("/checkout/success", handler.CreateEventHook)
	//endregion

	ps := api.Group("/pixelstreaming")
	ps.Get("/session/pending", middleware.ProtectedJwt(), handler.GetPendingSession)
	ps.Post("/session/request", handler.RequestPSSession)
	ps.Get("/session/:id", handler.GetPSSessionData)
	ps.Put("/session/:id", middleware.ProtectedJwt(), handler.UpdatePSSession)
	ps.Put("instance/status", middleware.ProtectedJwt(), handler.UpdatePSInstanceStatus)
	ps.Get("/launcher/latest", handler.GetLatestPSLauncher)

	//region NFT
	nft := api.Group("/nft")
	nft.Get("/assets", middleware.ProtectedJwt(), handler.GetRequesterNFTAssets)
	nft.Get("/:id", middleware.ProtectedJwt(), handler.GetRequesterNFTAsset)
	//endregion

	//region Apps and releases
	apps := api.Group("/apps")
	apps.Get("", middleware.ProtectedJwt(), handler.IndexApps)
	apps.Post("", middleware.ProtectedJwt(), handler.CreateApp)
	apps.Get("/public/:id", handler.GetAppPublic)
	apps.Get("/owned", middleware.ProtectedJwt(), handler.IndexOwnedApps)
	apps.Get("/:id", handler.GetApp)
	apps.Get("/:id/release-manager", middleware.ProtectedJwt(), handler.GetAppForReleaseManager)
	apps.Patch("/:id", middleware.ProtectedJwt(), handler.UpdateAppMetadata)
	apps.Get("/:id/releases", middleware.ProtectedJwt(), handler.IndexAppReleases)
	apps.Post("/:id/release", middleware.ProtectedJwt(), handler.NewAppRelease)
	apps.Patch("/release/:id", middleware.ProtectedJwt(), handler.UpdateAppRelease)
	apps.Get("/:id/releases/latest", handler.GetLatestRelease)
	apps.Get("/:id/launcher/latest", handler.GetLatestLauncher)
	apps.Get("/:id/images/identity", handler.GetAppIdentityImages)
	//endregion

	//region Releases
	releases := api.Group("/releases")
	releases.Get("", middleware.ProtectedJwt(), handler.IndexReleases)
	releases.Get("/latest", handler.GetLatestReleaseV2Public)
	releases.Get(":id", middleware.ProtectedJwt(), handler.GetRelease)
	//endregion

	//region Jobs
	jobs := api.Group("/jobs")
	jobs.Get("", middleware.ProtectedJwt(), handler.IndexJobs)
	jobs.Post("", middleware.ProtectedJwt(), handler.CreateJob)
	jobs.Post("/package", middleware.ProtectedJwt(), handler.CreatePackageJobs)
	jobs.Post("/release", middleware.ProtectedJwt(), handler.PublishReleaseForAllApps)
	jobs.Put("/reschedule", middleware.ProtectedJwt(), handler.RescheduleJob)
	jobs.Put("/cancel", middleware.ProtectedJwt(), handler.CancelJob)
	jobs.Get("/unclaimed", middleware.ProtectedJwt(), handler.GetUnclaimedJob)
	jobs.Patch("/:id/status", middleware.ProtectedJwt(), handler.UpdateJobStatus)
	jobs.Post("/:id/log", middleware.ProtectedJwt(), handler.ReportJobLog)
	//endregion

	//region Servers
	servers := api.Group("/servers")
	//servers.Get("", middleware.ProtectedJwt(), handler.IndexServers)
	servers.Get("/:id", middleware.ProtectedJwt(), handler.GetServer)
	servers.Get("/match/:id", middleware.ProtectedJwt(), handler.MatchServer)
	servers.Patch("/:id/heartbeat", handler.UpdateServerStatus)
	//endregion

	//region GameServer
	gameServers := api.Group("/game-servers")
	gameServers.Get("", middleware.ProtectedJwt(), handler.IndexGameServersV2)                                    // Index game servers
	gameServers.Get("/:id", middleware.ProtectedJwt(), handler.GetGameServerV2)                                   // Get single game server
	gameServers.Post("", middleware.ProtectedJwt(), handler.CreateGameServerV2)                                   // Create game server
	gameServers.Patch("/:id/status", middleware.ProtectedJwt(), handler.UpdateGameServerV2Status)                 // Update game server
	gameServers.Delete("/:id", middleware.ProtectedJwt(), handler.DeleteGameServerV2)                             // Delete game server
	gameServers.Post("/match", middleware.ProtectedJwt(), handler.MatchGameServerV2)                              // Match game server
	gameServers.Post("/:id/players", middleware.ProtectedJwt(), handler.AddPlayerToGameServerV2)                  // Add a player to game server
	gameServers.Patch("/:id/players/status", middleware.ProtectedJwt(), handler.UpdateGameServerV2PlayerStatus)   // Update player status on game server
	gameServers.Delete("/:id/players/:playerId", middleware.ProtectedJwt(), handler.RemovePlayerFromGameServerV2) // Remove a player from game server
	//endregion

	//region Launchers
	launchers := api.Group("/launchers")
	launchers.Get("/public/:id", handler.GetLauncherPublic)
	launchers.Get("/public/:id/apps", handler.IndexLauncherAppsPublic)
	launchers.Get("/public/:id/releases", handler.IndexLauncherReleasesPublic)
	launchers.Get("", middleware.ProtectedJwt(), handler.IndexLaunchers)
	launchers.Get("/:id", middleware.ProtectedJwt(), handler.GetLauncher)
	launchers.Post("", middleware.ProtectedJwt(), handler.CreateLauncher)
	launchers.Patch("/:id", middleware.ProtectedJwt(), handler.UpdateLauncher)
	launchers.Get("/:id/apps", middleware.ProtectedJwt(), handler.IndexLauncherApps)
	launchers.Get("/:id/releases", middleware.ProtectedJwt(), handler.IndexLauncherReleases)
	//endregion

	//region Stats
	stats := api.Group("/stats")
	stats.Get("", middleware.ProtectedJwt(), handler.GetStats)
	//endregion

	links := api.Group("/links")
	links.Get("sdk", handler.GetSdkLink)

	analytics := api.Group("/analytics")
	analytics.Get("/", middleware.ProtectedJwt(), handler.IndexAnalyticEvent)
	analytics.Post("/", middleware.ProtectedJwt(), handler.ReportEvent)
	analytics.Get("/app/:id", middleware.ProtectedJwt(), handler.IndexEntityAnalytic)

	//region AI
	ai := api.Group("/ai")
	ai.Post("/simple-fsm/states", middleware.ProtectedJwt(), handler.GetAiSimpleFsmStates)
	ai.Post("/tts-gc", middleware.ProtectedJwt(), handler.AiTextToSpeech)
	ai.Post("/tts/stream", middleware.ProtectedJwt(), handler.AiTextToSpeechElevenLabs)
	ai.Post("/tts", middleware.ProtectedJwt(), handler.AiTextToSpeechElevenLabsCached)
	ai.Post("/stt-gc", middleware.ProtectedJwt(), handler.AiSpeechToText)
	ai.Post("/stt", middleware.ProtectedJwt(), handler.AiSpeechToTextWhisper)
	ai.Post("/perception", middleware.ProtectedJwt(), handler.GetAiPerception)
	ai.Post("/cog/user", middleware.ProtectedJwt(), handler.CognitiveAiUser)
	ai.Post("/cog", middleware.ProtectedJwt(), handler.CognitiveAi)
	//endregion

	if os.Getenv("ENVIRONMENT") == "test" || os.Getenv("ENVIRONMENT") == "dev" {
		tests := api.Group("/tests")
		tests.Static("/static/", "./public", fiber.Static{Compress: false, ByteRange: false})
		tests.Static("/static/range/", "./public", fiber.Static{Compress: true, ByteRange: true})
		tests.Static("/static/compressed/", "./public", fiber.Static{Compress: true, ByteRange: false})
		tests.Static("/static/range-compressed/", "./public", fiber.Static{Compress: true, ByteRange: true})
	}
}
