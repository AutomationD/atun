package commands

/*
func newRootCmd(project *config.Project) *cobra.Command {
	//var izeLongDesc bytes.Buffer
	//err := template.Must(template.New("desc").Parse(izeDescTpl)).Execute(&izeLongDesc, struct {
	//	Message string
	//	Docs    string
	//	Version string
	//}{
	//	Message: pterm.White(pterm.Bold.Sprint("Welcome to IZE")),
	//	Docs:    pterm.Sprintf("%s %s", pterm.Blue("Docs:"), "https://ize.sh/docs"),
	//	Version: pterm.Sprintf("%s %s", pterm.Green("Version:"), version.FullVersionNumber()),
	//})
	//if err != nil {
	//	logrus.Fatal(err)
	//}
	//
	cmd := &cobra.Command{
		Use:              "ize",
		TraverseChildren: true,
		SilenceErrors:    true,
		Long:             "atun",
		Version:          "atun",
	}

	return cmd
}*/

func Execute() {
	println("Hello")
	//cfg := new(config.Project)
	//cmd := newRootCmd(cfg)
	//
	//cobra.OnInitialize(func() {
	//	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
	//		if len(f.Value.String()) != 0 {
	//			_ = viper.BindPFlag(strings.ReplaceAll(f.Name, "-", "_"), cmd.PersistentFlags().Lookup(f.Name))
	//		}
	//	})
	//
	//	config.InitConfig()
	//
	//	getConfig(cfg)
	//})
	//
	//if err := cmd.Execute(); err != nil {
	//	fmt.Println()
	//	pterm.Error.Println(err)
	//	os.Exit(1)
	//}
}

//func getConfig(cfg *config.Project) {
//	if slices.Contains(os.Args, "terraform") ||
//		slices.Contains(os.Args, "nvm") ||
//		!(slices.Contains(os.Args, "aws-profile") ||
//			slices.Contains(os.Args, "doc") ||
//			slices.Contains(os.Args, "completion") ||
//			slices.Contains(os.Args, "version") ||
//			slices.Contains(os.Args, "init") ||
//			slices.Contains(os.Args, "validate") ||
//			slices.Contains(os.Args, "config")) {
//		err := cfg.GetConfig()
//		if err != nil {
//			pterm.Error.Println(err)
//			os.Exit(1)
//		}
//		cfg.SettingAWSClient(cfg.Session)
//	}
//}
//
//func init() {
//	initLogger()
//	customizeDefaultPtermPrefix()
//}
//
//func initLogger() {
//	logrus.SetReportCaller(true)
//	logrus.SetFormatter(&logrus.TextFormatter{
//		PadLevelText:     true,
//		DisableTimestamp: true,
//		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
//			filename := path.Base(f.File)
//			return "", fmt.Sprintf(" %s:%d", filename, f.Line)
//		},
//	})
//}
//
//func customizeDefaultPtermPrefix() {
//	pterm.Info.Prefix = pterm.Prefix{
//		Text:  "ℹ",
//		Style: pterm.NewStyle(pterm.FgBlue),
//	}
//
//	pterm.Success.Prefix = pterm.Prefix{
//		Text:  "✓",
//		Style: pterm.NewStyle(pterm.FgGreen),
//	}
//
//	pterm.Error.Prefix = pterm.Prefix{
//		Text:  "✗",
//		Style: pterm.NewStyle(pterm.FgRed),
//	}
//
//	pterm.Warning.Prefix = pterm.Prefix{
//		Text:  "⚠",
//		Style: pterm.NewStyle(pterm.FgYellow),
//	}
//
//	pterm.DefaultSpinner.Sequence = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
//}
