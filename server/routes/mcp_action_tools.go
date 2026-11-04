package routes

import "github.com/gin-gonic/gin"

func actionToolDefs() []gin.H {
	id := gin.H{"type": "string", "description": "action_id returned by muster_ui_prepare_action."}
	write := gin.H{"readOnlyHint": false, "destructiveHint": true, "idempotentHint": false, "openWorldHint": false}
	return []gin.H{
		{"name": "muster_ui_prepare_action", "description": "App only. Check one closed operation and store an expiring review. Changes no muster until commit.", "annotations": write, "inputSchema": gin.H{
			"type": "object", "additionalProperties": false,
			"properties": gin.H{
				"operation":                  gin.H{"type": "string", "enum": []string{"edit_own_answer", "clear_own_answer", "decline_own_invitation", "finalize_time", "create_group", "update_group", "add_attendees", "remove_attendee"}, "description": "Closed operation to review. Nothing is saved until commit."},
				"event_id":                   gin.H{"type": "string", "description": "Muster the operation applies to, when the operation needs one."},
				"expected_state_fingerprint": gin.H{"type": "string", "description": "action_bases.write from the open view. The review is refused when it differs."},
				"payload":                    gin.H{"type": "object", "description": "Fields the operation will write. The review shows these before commit."},
			},
			"required": []string{"operation", "payload"},
		}, "_meta": gin.H{"ui": gin.H{"visibility": []string{"app"}}}},
		{"name": "muster_ui_commit_action", "description": "App only. Run a prepared action once. A second commit returns the stored receipt and does not run it again.", "annotations": write, "inputSchema": gin.H{
			"type": "object", "additionalProperties": false,
			"properties": gin.H{"action_id": id},
			"required":   []string{"action_id"},
		}, "_meta": gin.H{"ui": gin.H{"visibility": []string{"app"}}}},
		{"name": "muster_ui_get_action", "description": "App only. Read the receipt for one prepared action.", "annotations": gin.H{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}, "inputSchema": gin.H{
			"type": "object", "additionalProperties": false,
			"properties": gin.H{"action_id": id},
			"required":   []string{"action_id"},
		}, "_meta": gin.H{"ui": gin.H{"visibility": []string{"app"}}}},
	}
}
