package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kaelwang/go-Term/internal/store"
)

// ConnectionsList returns the caller's saved connections.
func ConnectionsList(c *gin.Context) {
	list, err := store.ListConnections(currentUser(c))
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", list)
}

// ConnectionsCreate saves a new connection.
func ConnectionsCreate(c *gin.Context) {
	var req store.Connection
	if err := c.ShouldBindJSON(&req); err != nil {
		respond(c, CodeBadParam, "bad params", nil)
		return
	}
	conn, err := store.CreateConnection(currentUser(c), &req)
	if err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"id": conn.ID})
}

// ConnectionsUpdate applies a partial update to an existing connection. The
// request body is bound into a generic map so that only the fields it mentions
// are changed; the store layer performs a read-modify-write (see UpdateConnection).
// This supports both full payloads (the edit form) and partial ones such as
// {"group_id": N} (batch move) without clearing unmentioned columns.
func ConnectionsUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	var fields map[string]interface{}
	if err := c.ShouldBindJSON(&fields); err != nil {
		respond(c, CodeBadParam, "bad params", nil)
		return
	}
	if len(fields) == 0 {
		// Nothing to change; treat as a successful no-op.
		respond(c, 0, "ok", nil)
		return
	}
	if err := store.UpdateConnection(currentUser(c), id, fields); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// ConnectionsDelete removes a connection.
func ConnectionsDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	if err := store.DeleteConnection(currentUser(c), id); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// GroupsList returns the caller's connection groups.
func GroupsList(c *gin.Context) {
	list, err := store.ListGroups(currentUser(c))
	if err != nil {
		respond(c, CodeTransferFail, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", list)
}

// GroupsCreate creates a new group.
func GroupsCreate(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		ParentID  *int   `json:"parent_id"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		respond(c, CodeBadParam, "name required", nil)
		return
	}
	g, err := store.CreateGroup(currentUser(c), req.Name, req.ParentID, req.SortOrder)
	if err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", gin.H{"id": g.ID})
}

// GroupsUpdate renames / reorders a group. The request body is bound into a
// generic map so that only the fields it mentions are changed (read-modify-write
// in the store layer). A rename payload {"name":"x"} therefore preserves the
// group's existing sort_order instead of resetting it to 0 (bug B3).
func GroupsUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	var fields map[string]interface{}
	if err := c.ShouldBindJSON(&fields); err != nil {
		respond(c, CodeBadParam, "bad params", nil)
		return
	}
	if len(fields) == 0 {
		// Nothing to change; treat as a successful no-op.
		respond(c, 0, "ok", nil)
		return
	}
	if err := store.UpdateGroup(currentUser(c), id, fields); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}

// GroupsDelete removes a group (its connections are detached).
func GroupsDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respond(c, CodeBadParam, "bad id", nil)
		return
	}
	if err := store.DeleteGroup(currentUser(c), id); err != nil {
		respond(c, CodeBadParam, err.Error(), nil)
		return
	}
	respond(c, 0, "ok", nil)
}
