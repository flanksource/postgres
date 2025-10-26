package helm

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Helm Chart Basic Tests", func() {
	Context("Chart Structure", func() {
		It("should have a valid Chart.yaml file", func() {
			chartPath := os.Getenv("CHART_PATH")
			if chartPath == "" {
				chartPath = "../chart"
			}

			chartYamlPath := filepath.Join(chartPath, "Chart.yaml")
			Expect(chartYamlPath).To(BeAnExistingFile())

			// Read and verify basic structure
			content, err := os.ReadFile(chartYamlPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("name: postgres-upgrade"))
			Expect(string(content)).To(ContainSubstring("apiVersion: v2"))
		})

		It("should have a values.yaml file", func() {
			chartPath := os.Getenv("CHART_PATH")
			if chartPath == "" {
				chartPath = "../chart"
			}

			valuesPath := filepath.Join(chartPath, "values.yaml")
			Expect(valuesPath).To(BeAnExistingFile())

			// Verify it contains expected sections
			content, err := os.ReadFile(valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("postgresql:"))
			Expect(string(content)).To(ContainSubstring("upgradeContainer:"))
			Expect(string(content)).To(ContainSubstring("database:"))
		})

		It("should have required templates", func() {
			chartPath := os.Getenv("CHART_PATH")
			if chartPath == "" {
				chartPath = "../chart"
			}

			templatesDir := filepath.Join(chartPath, "templates")

			// Check for essential template files
			requiredTemplates := []string{
				"statefulset.yaml",
				"service.yaml",
				"configmap.yaml",
				"secret.yaml",
			}

			for _, template := range requiredTemplates {
				templatePath := filepath.Join(templatesDir, template)
				Expect(templatePath).To(BeAnExistingFile(), "Missing template: %s", template)
			}
		})
	})

	Context("Helm Lint", func() {
		It("should pass helm lint", func() {
			Skip("Requires helm to be installed")

			chartPath := os.Getenv("CHART_PATH")
			if chartPath == "" {
				chartPath = "../chart"
			}

			// This would run: helm lint <chartPath>
			// For now, we skip this as it requires helm CLI
		})
	})
})
