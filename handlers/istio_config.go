package handlers

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/kiali/kiali/log"
	"github.com/kiali/kiali/services/business"
	"k8s.io/apimachinery/pkg/api/errors"
)

// SwitchRoute method
func SwitchRoute(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	namespace := params["namespace"]
	query := r.URL.Query()
	objects := ""
	if _, ok := query["objects"]; ok {
		objects = strings.ToLower(query.Get("objects"))
	}
	criteria := parseCriteria(namespace, objects)

	// Get business layer
	business, err := business.Get()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Services initialization error: "+err.Error())
		return
	}

	res, err := business.IstioConfig.SwitchRoute(criteria)

	if err != nil {
		log.Error(err)
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, res)
}

// IstioConfigList method
func IstioConfigList(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	namespace := params["namespace"]
	query := r.URL.Query()
	objects := ""
	if _, ok := query["objects"]; ok {
		objects = strings.ToLower(query.Get("objects"))
	}
	criteria := parseCriteria(namespace, objects)

	// Get business layer
	business, err := business.Get()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Services initialization error: "+err.Error())
		return
	}

	istioConfig, err := business.IstioConfig.GetIstioConfig(criteria)
	if err != nil {
		log.Error(err)
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, istioConfig)
}

func checkType(types []string, name string) bool {
	for _, typeName := range types {
		if typeName == name {
			return true
		}
	}
	return false
}

func parseCriteria(namespace string, objects string) business.IstioConfigCriteria {
	defaultInclude := objects == ""
	criteria := business.IstioConfigCriteria{}
	criteria.Namespace = namespace
	criteria.IncludeRouteRules = defaultInclude
	criteria.IncludeDestinationPolicies = defaultInclude
	criteria.IncludeVirtualServices = defaultInclude
	criteria.IncludeDestinationRules = defaultInclude
	criteria.IncludeRules = defaultInclude

	if defaultInclude {
		return criteria
	}

	types := strings.Split(objects, ",")
	if checkType(types, "routerules") {
		criteria.IncludeRouteRules = true
	}
	if checkType(types, "destinationpolicies") {
		criteria.IncludeDestinationPolicies = true
	}
	if checkType(types, "virtualservices") {
		criteria.IncludeVirtualServices = true
	}
	if checkType(types, "destinationrules") {
		criteria.IncludeDestinationRules = true
	}
	if checkType(types, "rules") {
		criteria.IncludeRules = true
	}
	return criteria
}

func IstioConfigDetails(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	namespace := params["namespace"]
	objectType := params["object_type"]
	object := params["object"]

	if !checkObjectType(objectType) {
		RespondWithError(w, http.StatusBadRequest, "Object type not found: "+objectType)
		return
	}

	// Get business layer
	business, err := business.Get()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Services initialization error: "+err.Error())
		return
	}

	istioConfigDetails, err := business.IstioConfig.GetIstioConfigDetails(namespace, objectType, object)
	if errors.IsNotFound(err) {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		RespondWithError(w, http.StatusInternalServerError, statusError.ErrStatus.Message)
		return
	} else if err != nil {
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, istioConfigDetails)
}

func IstioConfigValidations(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	namespace := params["namespace"]
	objectType := params["object_type"]
	object := params["object"]

	if !checkObjectType(objectType) {
		RespondWithError(w, http.StatusBadRequest, "Object type not found: "+objectType)
		return
	}

	// Get business layer
	business, err := business.Get()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Services initialization error: "+err.Error())
		return
	}

	istioConfigValidations, err := business.Validations.GetIstioObjectValidations(namespace, objectType, object)
	if errors.IsNotFound(err) {
		RespondWithError(w, http.StatusNotFound, err.Error())
		return
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		RespondWithError(w, http.StatusInternalServerError, statusError.ErrStatus.Message)
		return
	} else if err != nil {
		RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, istioConfigValidations)
}

func checkObjectType(objectType string) bool {
	switch objectType {
	case
		"routerules",
		"destinationpolicies",
		"virtualservices",
		"destinationrules",
		"rules":
		return true
	}
	return false
}
