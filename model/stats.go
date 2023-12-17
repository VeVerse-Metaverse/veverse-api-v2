package model

import (
	"context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"veverse-api/database"
)

type Stats struct {
	LatestWorld     *World   `json:"latestWorld,omitempty"`
	LatestPackage   *Package `json:"latestPackage,omitempty"`
	LatestUser      string   `json:"latestUser"`
	TotalUsers      int32    `json:"totalUsers"`
	OnlineUsers     int32    `json:"onlineUsers"`
	OnlineServers   int32    `json:"onlineServers"`
	TotalPackages   int32    `json:"totalPackages"`
	TotalWorlds     int32    `json:"totalWorlds"`
	TotalLikes      int32    `json:"totalLikes"`
	TotalObjects    int32    `json:"totalObjects"`
	TotalNFTs       int32    `json:"totalNFTs"`
	TotalPlaceables int32    `json:"totalPlaceables"`
}

type StatsRequestMetadata struct {
	Platform   string `json:"platform,omitempty"`   // SupportedPlatform (OS) of the pak file (Win64, Mac, Linux, IOS, Android)
	Deployment string `json:"deployment,omitempty"` // SupportedDeployment for the pak file (Server or Client)
}

func GetStats(ctx context.Context, requester *sm.User, platform string, deployment string) (stats *Stats, err error) {
	db := database.DB

	stats = new(Stats)

	//region Count Worlds
	{
		q := `SELECT COUNT(*) FROM spaces w`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalWorlds)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Packages
	{
		q := `SELECT COUNT(*) FROM mods p`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalPackages)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Likes
	{
		q := `SELECT COUNT(*) FROM likables l`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalLikes)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Objects
	{
		q := `SELECT COUNT(*) FROM objects o`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalObjects)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count NFTs
	{
		q := `SELECT COUNT(*) FROM nft_assets n`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalNFTs)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Placeables
	{
		q := `SELECT COUNT(*) FROM placeables p`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalPlaceables)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Users
	{
		q := `SELECT COUNT(*) FROM users u`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.TotalUsers)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Online Players
	{
		q := `SELECT COUNT(*) FROM online_players p`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.OnlineUsers)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Count Online Servers
	{
		q := `SELECT COUNT(*) FROM servers s WHERE s.status = 'online' AND age(now(), s.updated_at) < interval '2 minutes'`
		row := db.QueryRow(ctx, q)

		err = row.Scan(&stats.OnlineServers)
		if err != nil {
			return nil, err
		}
	}
	//endregion

	//region Latest World
	stats.LatestWorld, err = GetLatestWorldForAdminWithPak(ctx, requester, platform, deployment)
	if err != nil {
		return nil, err
	}
	//endregion

	//region Latest Package
	stats.LatestPackage, err = GetLatestPackageForAdminWithPak(ctx, platform, deployment)
	if err != nil {
		return nil, err
	}
	//endregion

	return stats, err
}
