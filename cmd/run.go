package cmd

import (
	"fmt"
	"log"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/agent"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a dropstep workflow",
	Run: func(cmd *cobra.Command, args []string) {
		jobs, err := internal.LoadAllJobs("jobs")
		if err != nil {
			log.Fatal("Failed to load jobs:", err)
		}

		for _, job := range jobs {
			fmt.Println("Running job:", job.ID)
			switch job.Tool {
			case "browseruse":
				err := runner.RunBrowserUse(job)
				if err != nil {
					log.Printf("Job %s failed: %v\n", job.ID, err)
				}
			default:
				log.Printf("Unknown tool %s in job %s\n", job.Tool, job.ID)
			}
		}
	},
}
