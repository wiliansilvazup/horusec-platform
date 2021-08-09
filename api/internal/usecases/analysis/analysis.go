// Copyright 2021 ZUP IT SERVICOS EM TECNOLOGIA E INOVACAO SA
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analysis

import (
	"context"
	netHTTP "net/http"
	"strings"

	"github.com/ZupIT/horusec-devkit/pkg/services/tracer"

	"github.com/opentracing/opentracing-go"

	analysisv1 "github.com/ZupIT/horusec-platform/api/internal/entities/analysis_v1"

	"github.com/ZupIT/horusec-devkit/pkg/enums/confidence"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"

	"github.com/ZupIT/horusec-devkit/pkg/utils/parser/enums"

	analysisEntity "github.com/ZupIT/horusec-devkit/pkg/entities/analysis"
	"github.com/ZupIT/horusec-devkit/pkg/entities/cli"
	"github.com/ZupIT/horusec-devkit/pkg/entities/vulnerability"
	analysisEnum "github.com/ZupIT/horusec-devkit/pkg/enums/analysis"
	"github.com/ZupIT/horusec-devkit/pkg/enums/languages"
	"github.com/ZupIT/horusec-devkit/pkg/enums/severities"
	"github.com/ZupIT/horusec-devkit/pkg/enums/tools"
	vulnerabilityEnum "github.com/ZupIT/horusec-devkit/pkg/enums/vulnerability"
	"github.com/ZupIT/horusec-devkit/pkg/utils/parser"
)

type Interface interface {
	DecodeAnalysisDataFromIoRead(r *netHTTP.Request) (analysisData *cli.AnalysisData, err error)
}

type UseCases struct {
	versionsOfV1 []string
}

func NewAnalysisUseCases() Interface {
	return &UseCases{
		versionsOfV1: []string{"v1.7.0", "v1.8.0", "v1.8.1", "v1.8.2", "v1.8.3", "v1.8.4",
			"v1.9.0", "v1.10.0", "v1.10.1", "v1.10.2", "v1.10.3"},
	}
}

func (au *UseCases) DecodeAnalysisDataFromIoRead(r *netHTTP.Request) (
	analysisData *cli.AnalysisData, err error) {
	span, ctx := opentracing.StartSpanFromContext(r.Context(), "DecodeAnalysisDataFromIoRead")
	defer span.Finish()
	if r.Body == nil {
		tracer.SetSpanError(span, enums.ErrorBodyEmpty)
		return nil, enums.ErrorBodyEmpty
	}
	analysisData, err = au.parseBodyToAnalysis(r.WithContext(ctx))
	if err != nil {
		tracer.SetSpanError(span, err)
		return nil, err
	}
	return analysisData, au.validateAnalysisData(ctx, analysisData)
}

func (au *UseCases) parseBodyToAnalysis(r *netHTTP.Request) (analysisData *cli.AnalysisData, err error) {
	span, _ := opentracing.StartSpanFromContext(r.Context(), "parseBodyToAnalysis")
	defer span.Finish()
	if au.isVersion1(r.Header.Get("X-Horusec-CLI-Version")) {
		analysisDataV1 := &analysisv1.AnalysisCLIDataV1{}
		if err = parser.ParseBodyToEntity(r.Body, analysisDataV1); err != nil {
			tracer.SetSpanError(span, err)
			return nil, err
		}
		analysisData = analysisDataV1.ParseDataV1ToV2()
	} else if err = parser.ParseBodyToEntity(r.Body, &analysisData); err != nil {
		tracer.SetSpanError(span, err)
		return nil, err
	}
	return analysisData, nil
}

func (au *UseCases) validateAnalysisData(ctx context.Context, analysisData *cli.AnalysisData) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "validateAnalysisData")
	defer span.Finish()
	err := validation.ValidateStruct(analysisData,
		validation.Field(&analysisData.Analysis, validation.Required),
	)
	if err != nil {
		tracer.SetSpanError(span, err)
		return err
	}
	return au.validateAnalysisToCLIv2(ctx, analysisData.Analysis)
}

func (au *UseCases) validateAnalysisToCLIv2(ctx context.Context, analysis *analysisEntity.Analysis) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "validateAnalysisToCLIv2")
	defer span.Finish()
	return validation.ValidateStruct(analysis,
		validation.Field(&analysis.ID, validation.Required, is.UUID),
		validation.Field(&analysis.Status, validation.Required,
			validation.In(au.sliceAnalysisStatus()...)),
		validation.Field(&analysis.CreatedAt, validation.Required, validation.NilOrNotEmpty),
		validation.Field(&analysis.FinishedAt, validation.Required, validation.NilOrNotEmpty),
		validation.Field(&analysis.AnalysisVulnerabilities,
			validation.By(au.validateVulnerabilities(ctx, analysis.AnalysisVulnerabilities))),
	)
}

func (au *UseCases) validateVulnerabilities(ctx context.Context,
	analysisVulnerabilities []analysisEntity.AnalysisVulnerabilities) validation.RuleFunc {
	span, _ := opentracing.StartSpanFromContext(ctx, "validateVulnerabilities")
	defer span.Finish()
	return func(value interface{}) error {
		for key := range analysisVulnerabilities {
			if err := au.setupValidationVulnerabilities(&analysisVulnerabilities[key].Vulnerability); err != nil {
				tracer.SetSpanError(span, err)
				return err
			}
		}
		return nil
	}
}

func (au *UseCases) setupValidationVulnerabilities(vul *vulnerability.Vulnerability) error {
	return validation.ValidateStruct(vul,
		validation.Field(&vul.SecurityTool, validation.Required, validation.In(au.sliceTools()...)),
		validation.Field(&vul.VulnHash, validation.Required),
		validation.Field(&vul.Confidence, validation.Required, validation.In(au.sliceConfidence()...)),
		validation.Field(&vul.Language, validation.Required, validation.In(au.sliceLanguages()...)),
		validation.Field(&vul.Severity, validation.Required, validation.In(au.sliceSeverities()...)),
		validation.Field(&vul.Type, validation.Required, validation.In(au.sliceVulnerabilitiesType()...)),
	)
}

// nolint
func (au *UseCases) sliceTools() []interface{} {
	return []interface{}{
		tools.GoSec,
		tools.SecurityCodeScan,
		tools.GitLeaks,
		tools.Brakeman,
		tools.NpmAudit,
		tools.Safety,
		tools.Bandit,
		tools.YarnAudit,
		tools.TfSec,
		tools.HorusecEngine,
		tools.Semgrep,
		tools.Flawfinder,
		tools.PhpCS,
		tools.ShellCheck,
		tools.BundlerAudit,
		tools.Sobelow,
		tools.MixAudit,
		tools.OwaspDependencyCheck,
		tools.DotnetCli,
		tools.Nancy,
	}
}

// nolint
func (au *UseCases) sliceLanguages() []interface{} {
	return []interface{}{
		languages.Go,
		languages.CSharp,
		languages.Dart,
		languages.Ruby,
		languages.Python,
		languages.Java,
		languages.Kotlin,
		languages.Javascript,
		languages.Leaks,
		languages.HCL,
		languages.PHP,
		languages.Typescript,
		languages.C,
		languages.HTML,
		languages.Generic,
		languages.Yaml,
		languages.Shell,
		languages.Elixir,
		languages.Nginx,
		languages.Swift,
	}
}

func (au *UseCases) sliceSeverities() []interface{} {
	return []interface{}{
		severities.Critical,
		severities.High,
		severities.Medium,
		severities.Low,
		severities.Info,
		severities.Unknown,
	}
}

func (au *UseCases) sliceVulnerabilitiesType() []interface{} {
	return []interface{}{
		vulnerabilityEnum.FalsePositive,
		vulnerabilityEnum.RiskAccepted,
		vulnerabilityEnum.Vulnerability,
		vulnerabilityEnum.Corrected,
	}
}

func (au *UseCases) sliceAnalysisStatus() []interface{} {
	return []interface{}{
		analysisEnum.Running,
		analysisEnum.Success,
		analysisEnum.Error,
	}
}

func (au *UseCases) sliceConfidence() []interface{} {
	return []interface{}{
		confidence.High,
		confidence.Medium,
		confidence.Low,
	}
}

func (au *UseCases) isVersion1(versionSent string) bool {
	for _, versionV1 := range au.versionsOfV1 {
		if strings.EqualFold(versionV1, versionSent) {
			return true
		}
	}
	return false
}
