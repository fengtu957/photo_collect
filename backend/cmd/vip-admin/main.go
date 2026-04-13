package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	mongoURI := os.Getenv("MONGODB_URI")
	d, err := data.NewData(mongoURI)
	if err != nil {
		log.Fatal(err)
	}

	vipRepo := data.NewVIPRepo(d)
	if err := vipRepo.EnsureIndexes(context.Background()); err != nil {
		log.Fatal(err)
	}
	vipUC := biz.NewVIPUsecase(vipRepo)

	switch os.Args[1] {
	case "create-codes":
		runCreateCodes(vipUC, os.Args[2:])
	case "grant-vip":
		runGrantVIP(vipUC, os.Args[2:])
	case "disable-code":
		runDisableCode(vipUC, os.Args[2:])
	case "show-code":
		runShowCode(vipUC, os.Args[2:])
	case "show-user":
		runShowUser(vipUC, os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func runCreateCodes(vipUC *biz.VIPUsecase, args []string) {
	cmd := flag.NewFlagSet("create-codes", flag.ExitOnError)
	plan := cmd.String("plan", "vip_month", "")
	days := cmd.Int("days", 30, "")
	count := cmd.Int("count", 1, "")
	expireAtText := cmd.String("expire-at", "", "")
	remark := cmd.String("remark", "", "")
	cmd.Parse(args)

	var expireAt time.Time
	if *expireAtText != "" {
		parsed, err := time.Parse(time.RFC3339, *expireAtText)
		if err != nil {
			log.Fatal(err)
		}
		expireAt = parsed
	}

	codes, err := vipUC.CreateActivationCodes(context.Background(), biz.CreateActivationCodesRequest{
		PlanCode:    *plan,
		DurationDay: *days,
		Count:       *count,
		ExpireAt:    expireAt,
		Remark:      *remark,
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, code := range codes {
		fmt.Println(code.Code)
	}
}

func runGrantVIP(vipUC *biz.VIPUsecase, args []string) {
	cmd := flag.NewFlagSet("grant-vip", flag.ExitOnError)
	userID := cmd.String("user-id", "", "")
	plan := cmd.String("plan", "vip_manual", "")
	days := cmd.Int("days", 30, "")
	remark := cmd.String("remark", "", "")
	cmd.Parse(args)

	membership, err := vipUC.GrantVIP(context.Background(), *userID, *plan, *days, "manual", *remark)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("user=%s expire_at=%s\n", membership.UserID, membership.ExpireAt.Format(time.RFC3339))
}

func runDisableCode(vipUC *biz.VIPUsecase, args []string) {
	cmd := flag.NewFlagSet("disable-code", flag.ExitOnError)
	code := cmd.String("code", "", "")
	cmd.Parse(args)

	if err := vipUC.DisableCode(context.Background(), *code); err != nil {
		log.Fatal(err)
	}

	fmt.Println("ok")
}

func runShowCode(vipUC *biz.VIPUsecase, args []string) {
	cmd := flag.NewFlagSet("show-code", flag.ExitOnError)
	code := cmd.String("code", "", "")
	cmd.Parse(args)

	result, err := vipUC.GetCode(context.Background(), *code)
	if err != nil {
		log.Fatal(err)
	}
	if result == nil {
		fmt.Println("not found")
		return
	}

	fmt.Printf("code=%s status=%s plan=%s days=%d used_by=%s expire_at=%s\n", result.Code, result.Status, result.PlanCode, result.DurationDay, result.UsedBy, result.ExpireAt.Format(time.RFC3339))
}

func runShowUser(vipUC *biz.VIPUsecase, args []string) {
	cmd := flag.NewFlagSet("show-user", flag.ExitOnError)
	userID := cmd.String("user-id", "", "")
	cmd.Parse(args)

	entitlements, err := vipUC.GetUserEntitlements(context.Background(), *userID)
	if err != nil {
		log.Fatal(err)
	}

	expireAt := ""
	if entitlements.ExpireAt != nil {
		expireAt = entitlements.ExpireAt.Format(time.RFC3339)
	}
	fmt.Printf("user=%s is_vip=%v plan=%s expire_at=%s max_active_tasks=%d max_submissions_per_task=%d can_use_ai_analysis=%v\n",
		*userID,
		entitlements.IsVIP,
		entitlements.PlanCode,
		expireAt,
		entitlements.Limits.MaxActiveTasks,
		entitlements.Limits.MaxSubmissionsPerTask,
		entitlements.Limits.CanUseAIAnalysis,
	)
}

func printUsage() {
	fmt.Println("vip-admin create-codes --plan vip_month --days 30 --count 10 [--expire-at RFC3339] [--remark text]")
	fmt.Println("vip-admin grant-vip --user-id OPENID [--plan vip_manual] [--days 30] [--remark text]")
	fmt.Println("vip-admin disable-code --code CODE")
	fmt.Println("vip-admin show-code --code CODE")
	fmt.Println("vip-admin show-user --user-id OPENID")
}
