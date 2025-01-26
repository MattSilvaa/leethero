package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MattSilvaa/leethero/internal/config"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type LeetHero struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *config.Config
}

func New(cfg *config.Config) (*LeetHero, error) {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	}

	if cfg.Headless {
		opts = append(opts, chromedp.Headless)
	}

	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, _ = chromedp.NewContext(ctx)

	return &LeetHero{
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
	}, nil
}

func (h *LeetHero) setCookie(ctx context.Context) error {
	fmt.Println("Setting LeetCode session cookie...")
	return chromedp.Run(ctx,
		chromedp.Navigate("https://leetcode.com"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			expr := cdp.TimeSinceEpoch(time.Now().Add(14 * 24 * time.Hour))
			cookie := []*network.CookieParam{
				{
					Name:     "LEETCODE_SESSION",
					Value:    h.config.LeetCodeSession,
					Domain:   ".leetcode.com",
					Path:     "/",
					Expires:  &expr,
					Secure:   true,
					HTTPOnly: true,
				},
			}
			return network.SetCookies(cookie).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var signInExists bool
			if err := chromedp.Evaluate(`!!document.querySelector('#navbar_sign_in_button')`, &signInExists).Do(ctx); err != nil {
				return err
			}
			if signInExists {
				return fmt.Errorf("cookie authentication failed")
			}
			fmt.Println("Successfully authenticated")
			return nil
		}),
	)
}

func (h *LeetHero) solveProblem(slug string) error {
	fmt.Printf("Solving problem: %s\n", slug)
	submitButton := `[data-e2e-locator="console-submit-button"]`

	var currentLang string
	return chromedp.Run(h.ctx,
		chromedp.Navigate(fmt.Sprintf("https://leetcode.com/problems/%s/", slug)),
		chromedp.Sleep(h.config.Delay),

		chromedp.Text(`button.group`, &currentLang),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if currentLang != "Python3" {
				if err := chromedp.Click(`button.group`).Do(ctx); err != nil {
					return err
				}
				if err := chromedp.WaitVisible(`//div[contains(text(), "Python3")]`).Do(ctx); err != nil {
					return err
				}
				if err := chromedp.Click(`//div[contains(text(), "Python3")]`).Do(ctx); err != nil {
					return err
				}
			}
			return nil
		}),

		chromedp.Sleep(h.config.Delay),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Use JavaScript to completely replace the editor content
			script := fmt.Sprintf(`
                var editor = monaco.editor.getModels()[0];
                editor.setValue(`+"`%s`"+`);
            `, Solutions[slug])

			return chromedp.Evaluate(script, nil).Do(ctx)
		}),

		chromedp.Sleep(h.config.Delay),
		chromedp.WaitVisible(submitButton),
		chromedp.Click(submitButton),
		chromedp.Sleep(h.config.Delay),
	)
}

func (h *LeetHero) Run() error {
	defer h.cancel()
	if err := h.setCookie(h.ctx); err != nil {
		return fmt.Errorf("failed to set cookie: %v", err)
	}

	for _, problem := range h.config.Problems {
		log.Printf("Solving problem: %s", problem)
		if err := h.solveProblem(problem); err != nil {
			log.Printf("Failed to solve %s: %v", problem, err)
			continue
		}
		log.Printf("Successfully solved: %s", problem)
	}

	return nil
}
