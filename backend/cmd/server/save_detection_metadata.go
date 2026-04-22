package main

import "strings"

func isStoredFallbackReason(reason string) bool {
	return strings.EqualFold(strings.TrimSpace(reason), "stored system fallback")
}

func isTrustedSystemEvidence(evidence saveDetectionEvidence, _ string) bool {
	if evidence.StoredTrusted || evidence.HelperTrusted || evidence.Payload || evidence.PathHint || evidence.FormatHint {
		return true
	}
	return false
}

func mergeSystemDetectionMetadata(existing any, detection saveSystemDetectionResult) any {
	detectionMeta := map[string]any{
		"slug":          strings.TrimSpace(detection.Slug),
		"confidence":    detection.Confidence,
		"reason":        strings.TrimSpace(detection.Reason),
		"noise":         detection.Noise,
		"trustedSystem": isTrustedSystemEvidence(detection.Evidence, detection.Reason),
		"evidence": map[string]any{
			"declared":      detection.Evidence.Declared,
			"helperTrusted": detection.Evidence.HelperTrusted,
			"payload":       detection.Evidence.Payload,
			"pathHint":      detection.Evidence.PathHint,
			"formatHint":    detection.Evidence.FormatHint,
			"storedTrusted": detection.Evidence.StoredTrusted,
			"titleHint":     detection.Evidence.TitleHint,
		},
	}

	if existingMap, ok := existing.(map[string]any); ok {
		merged := make(map[string]any, len(existingMap)+1)
		for key, value := range existingMap {
			merged[key] = value
		}
		rsm := map[string]any{}
		if rawRSM, ok := merged["rsm"].(map[string]any); ok {
			for key, value := range rawRSM {
				rsm[key] = value
			}
		}
		rsm["systemDetection"] = detectionMeta
		merged["rsm"] = rsm
		return merged
	}

	if existing == nil {
		return map[string]any{
			"rsm": map[string]any{
				"systemDetection": detectionMeta,
			},
		}
	}

	return map[string]any{
		"rsm": map[string]any{
			"systemDetection": detectionMeta,
		},
		"sourceMetadata": existing,
	}
}

func metadataHasTrustedSystemEvidence(metadata any) bool {
	root, ok := metadata.(map[string]any)
	if !ok {
		return false
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return false
	}
	detection, ok := rsm["systemDetection"].(map[string]any)
	if !ok {
		return false
	}
	reason, _ := detection["reason"].(string)
	evidence, ok := detection["evidence"].(map[string]any)
	if !ok {
		if trusted, ok := detection["trustedSystem"].(bool); ok {
			return trusted && !isStoredFallbackReason(reason)
		}
		return false
	}
	detectionEvidence := saveDetectionEvidence{}
	if value, ok := evidence["declared"].(bool); ok {
		detectionEvidence.Declared = value
	}
	if value, ok := evidence["helperTrusted"].(bool); ok {
		detectionEvidence.HelperTrusted = value
	}
	if value, ok := evidence["payload"].(bool); ok {
		detectionEvidence.Payload = value
	}
	if value, ok := evidence["pathHint"].(bool); ok {
		detectionEvidence.PathHint = value
	}
	if value, ok := evidence["formatHint"].(bool); ok {
		detectionEvidence.FormatHint = value
	}
	if value, ok := evidence["storedTrusted"].(bool); ok {
		detectionEvidence.StoredTrusted = value
	}
	return isTrustedSystemEvidence(detectionEvidence, reason)
}
