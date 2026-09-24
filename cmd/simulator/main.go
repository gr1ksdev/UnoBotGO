package main

import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/malbs/UnoGoBot/internal/simulation"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, input io.Reader, output, errorOutput io.Writer) int {
	flags := flag.NewFlagSet("simulator", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	players := flags.Int("players", 0, "quantidade de jogadores (2 a 10)")
	modeValue := flags.String("mode", "", "modo: classico ou caseiro")
	seed := flags.Uint64("seed", 0, "semente reproduzível (0 gera uma nova)")
	maxActions := flags.Int("max-actions", simulation.DefaultMaxActions, "limite de ações da partida")
	reportPath := flags.String("output", "", "caminho do relatório Markdown")
	quiet := flags.Bool("quiet", false, "omite o acompanhamento de cada jogada")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	reader := bufio.NewReader(input)
	if *players == 0 {
		value, err := prompt(reader, output, "Quantidade de jogadores (2 a 10): ")
		if err != nil {
			fmt.Fprintf(errorOutput, "erro ao ler jogadores: %v\n", err)
			return 2
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			fmt.Fprintf(errorOutput, "quantidade de jogadores inválida: %q\n", value)
			return 2
		}
		*players = parsed
	}
	if strings.TrimSpace(*modeValue) == "" {
		value, err := prompt(reader, output, "Modo (1=classico ou 2=caseiro): ")
		if err != nil {
			fmt.Fprintf(errorOutput, "erro ao ler modo: %v\n", err)
			return 2
		}
		*modeValue = value
	}
	mode, err := simulation.ParseMode(*modeValue)
	if err != nil {
		fmt.Fprintln(errorOutput, err)
		return 2
	}
	if *seed == 0 {
		*seed = newSeed()
	}
	config := simulation.Config{
		Players: *players, Mode: mode, Seed: *seed, MaxActions: *maxActions,
		Output: *reportPath, Quiet: *quiet,
	}
	if err := config.Validate(); err != nil {
		fmt.Fprintln(errorOutput, err)
		return 2
	}

	fmt.Fprintf(output, "\nSimulação iniciada: %d jogadores, modo %s, semente %d.\n", config.Players, config.Mode.Label(), config.Seed)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	observer := simulation.Observer(nil)
	if !config.Quiet {
		observer = func(step simulation.Step) {
			if step.Setup {
				return
			}
			fmt.Fprintf(output, "[%04d] %s\n", step.Number, simulation.DescribeStep(step))
			for _, explanation := range simulation.ExplainStep(step) {
				fmt.Fprintf(output, "       ↳ %s\n", explanation)
			}
		}
	}
	result, runErr := simulation.Run(ctx, config, observer)
	path := config.Output
	if path == "" {
		path = simulation.DefaultReportPath(result.StartedAt, config.Seed)
	}
	if err := simulation.WriteReport(path, result); err != nil {
		fmt.Fprintf(errorOutput, "não foi possível salvar o relatório: %v\n", err)
		return 2
	}

	fmt.Fprintf(output, "\nRelatório salvo em %s\n", path)
	if result.Completed {
		fmt.Fprintf(output, "Partida concluída em %d ações de jogo.\n", simulation.CollectStats(result).GameplayActions)
		for _, placement := range result.FinalState.Placements {
			fmt.Fprintf(output, "%dº — %s\n", placement.Position, simulation.PlayerName(placement.PlayerID))
		}
	}
	if runErr != nil {
		fmt.Fprintf(errorOutput, "simulação interrompida: %v\n", runErr)
		return 1
	}
	return 0
}

func prompt(reader *bufio.Reader, output io.Writer, message string) (string, error) {
	fmt.Fprint(output, message)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", io.ErrUnexpectedEOF
	}
	return value, nil
}

func newSeed() uint64 {
	var raw [8]byte
	if _, err := cryptorand.Read(raw[:]); err == nil {
		seed := binary.LittleEndian.Uint64(raw[:])
		if seed != 0 {
			return seed
		}
	}
	return uint64(time.Now().UnixNano())
}
