// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vulnerable

import (
	"fmt"
	"github.com/goharbor/harbor/src/common/dao"
	"github.com/goharbor/harbor/src/common/models"
	"github.com/goharbor/harbor/src/common/utils/log"
	"github.com/goharbor/harbor/src/core/config"
	"github.com/goharbor/harbor/src/core/middlewares/util"
	"net/http"
)

type vulnerableHandler struct {
	next http.Handler
}

// New ...
func New(next http.Handler) http.Handler {
	return &vulnerableHandler{
		next: next,
	}
}

// ServeHTTP ...
func (vh vulnerableHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	imgRaw := req.Context().Value(util.ImageInfoCtxKey)
	if imgRaw == nil || !config.WithClair() {
		vh.next.ServeHTTP(rw, req)
		return
	}
	img, _ := req.Context().Value(util.ImageInfoCtxKey).(util.ImageInfo)
	if img.Digest == "" {
		vh.next.ServeHTTP(rw, req)
		return
	}
	projectVulnerableEnabled, projectVulnerableSeverity := util.GetPolicyChecker().VulnerablePolicy(img.ProjectName)
	if !projectVulnerableEnabled {
		vh.next.ServeHTTP(rw, req)
		return
	}
	overview, err := dao.GetImgScanOverview(img.Digest)
	if err != nil {
		log.Errorf("failed to get ImgScanOverview with repo: %s, reference: %s, digest: %s. Error: %v", img.Repository, img.Reference, img.Digest, err)
		http.Error(rw, util.MarshalError("PROJECT_POLICY_VIOLATION", "Failed to get ImgScanOverview."), http.StatusPreconditionFailed)
		return
	}
	// severity is 0 means that the image fails to scan or not scanned successfully.
	if overview == nil || overview.Sev == 0 {
		http.Error(rw, util.MarshalError("PROJECT_POLICY_VIOLATION", "Cannot get the image severity."), http.StatusPreconditionFailed)
		return
	}
	imageSev := overview.Sev
	if imageSev >= int(projectVulnerableSeverity) {
		log.Debugf("the image severity: %q is higher then project setting: %q, failing the response.", models.Severity(imageSev), projectVulnerableSeverity)
		http.Error(rw, util.MarshalError("PROJECT_POLICY_VIOLATION", fmt.Sprintf("The severity of vulnerability of the image: %q is equal or higher than the threshold in project setting: %q.", models.Severity(imageSev), projectVulnerableSeverity)), http.StatusPreconditionFailed)
		return
	}
	vh.next.ServeHTTP(rw, req)
}