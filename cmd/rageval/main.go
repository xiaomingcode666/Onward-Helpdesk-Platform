package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"remotehelpdesk/internal/ai/rag/eval"
	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/logx"
)

func main() {
	if err := run(); err != nil {
		slog.Error("run rag eval failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	suitePath := flag.String("suite", "testdata/rag-eval/customer_robot_seed.yaml", "path to eval suite yaml")
	refsPath := flag.String("refs", "testdata/rag-eval/refs.local.yaml", "path to local refs yaml")
	topK := flag.Int("topk", 10, "retrieval topK for each case")
	scoreThreshold := flag.Float64("score-threshold", 0.3, "retrieval score threshold")
	rerankTopN := flag.Int("rerank-topn", 0, "override rerank topN, 0 keeps config auto behavior")
	contextMaxTokens := flag.Int("context-max-tokens", 4000, "context token budget for each case")
	withAnswer := flag.Bool("with-answer", false, "generate answer text and evaluate answer assertions")
	jsonOutput := flag.Bool("json", false, "render full report as json")
	flag.Parse()

	if err := initRuntime(*configPath); err != nil {
		return err
	}

	suite, err := eval.LoadSuite(*suitePath)
	if err != nil {
		return fmt.Errorf("load suite failed: %w", err)
	}
	refs, err := eval.LoadRefs(*refsPath)
	if err != nil {
		return fmt.Errorf("load refs failed: %w", err)
	}

	report, err := eval.NewEvaluator().Evaluate(context.Background(), suite, refs, eval.RunOptions{
		TopK:             *topK,
		ScoreThreshold:   *scoreThreshold,
		RerankTopN:       *rerankTopN,
		ContextMaxTokens: *contextMaxTokens,
		Channel:          "rag_eval",
		Scene:            "rag_eval",
		GenerateAnswer:   *withAnswer,
	})
	if err != nil {
		return err
	}

	if *jsonOutput {
		return eval.RenderJSON(os.Stdout, report)
	}
	return eval.RenderText(os.Stdout, report)
}

func initRuntime(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	config.SetCurrent(cfg)
	i18nx.SetDefaultLocale(cfg.LanguageOrDefault())
	logx.Init(logx.Config{
		Level:     cfg.Logger.Level,
		Format:    cfg.Logger.Format,
		AddSource: cfg.Logger.AddSource,
	})
	if _, err := bootstrap.InitDB(cfg.DB); err != nil {
		return fmt.Errorf("init db failed: %w", err)
	}
	if err := vectordb.Init(&cfg.VectorDB); err != nil {
		return fmt.Errorf("init vector db failed: %w", err)
	}
	return nil
}
