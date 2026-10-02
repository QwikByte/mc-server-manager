package access

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strconv"

	"github.com/QwikByte/mc-server-manager/internal/master/auth"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

var (
	errBuiltin   = httpapi.Errorf(http.StatusBadRequest, "The Administrators group always has every permission; only its members can change.")
	errEscalate  = httpapi.Errorf(http.StatusForbidden, "You can only grant permissions that you have yourself, within your own scope.")
	errStronger  = httpapi.Errorf(http.StatusForbidden, "This user has permissions that you don't have, so only someone with more permissions can manage them.")
	errLastAdmin = httpapi.Errorf(http.StatusConflict, "Keep at least one enabled administrator.")
	errSelf      = httpapi.Errorf(http.StatusBadRequest, "You can't disable or delete your own account.")
)

// Handler serves the groups and the user management.
type Handler struct {
	svc   *Service
	users *auth.Service
}

func NewHandler(svc *Service, users *auth.Service) *Handler { return &Handler{svc: svc, users: users} }

func (h *Handler) Register(m Mux) {
	m.Handle("GET /api/access/me", SignedIn, func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, From(r.Context()).view())
	})
	m.Handle("GET /api/access/permissions", SignedIn, func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, Catalog)
	})
	m.Handle("GET /api/groups", Everywhere(UsersView), func(w http.ResponseWriter, r *http.Request) {
		groups, err := h.svc.Groups(r.Context())
		write(w, r, http.StatusOK, groups, err)
	})
	m.Handle("GET /api/groups/{id}", Everywhere(UsersView), func(w http.ResponseWriter, r *http.Request) {
		group, err := h.svc.Group(r.Context(), r.PathValue("id"))
		write(w, r, http.StatusOK, group, err)
	})
	m.Handle("POST /api/groups", Everywhere(GroupsManage), h.saveGroup)
	m.Handle("PUT /api/groups/{id}", Everywhere(GroupsManage), h.saveGroup)
	m.Handle("DELETE /api/groups/{id}", Everywhere(GroupsManage), h.deleteGroup)

	m.Handle("GET /api/users", Everywhere(UsersView), h.listUsers)
	m.Handle("POST /api/users", Everywhere(UsersManage), h.invite)
	m.Handle("PUT /api/users/{id}", Everywhere(UsersManage), h.updateUser)
	m.Handle("DELETE /api/users/{id}", Everywhere(UsersManage), h.deleteUser)
	m.Handle("POST /api/users/{id}/setup-link", Everywhere(UsersManage), h.setupLink)
}

// saveGroup creates a group, or changes the one with the ID of the path. Only permissions
// the user has can be granted, and only groups within them changed.
func (h *Handler) saveGroup(w http.ResponseWriter, r *http.Request) {
	var in GroupInput
	if err := httpapi.ReadJSON(w, r, &in); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx, actor := r.Context(), From(r.Context())
	err := in.check()
	if err == nil && !actor.Covers(Group{Permissions: in.Permissions, AllServers: in.AllServers, Targets: in.Targets}.grants()) {
		err = errEscalate
	}
	var group Group
	id := r.PathValue("id")
	if err == nil && id != "" {
		group, err = h.svc.Group(ctx, id)
		switch {
		case err != nil:
		case group.Builtin:
			err = errBuiltin
		case !actor.Covers(group.grants()):
			err = errEscalate
		}
	}
	status := http.StatusOK
	if err == nil && id == "" {
		group, err = h.svc.createGroup(ctx, in)
		status = http.StatusCreated
	} else if err == nil {
		group, err = h.svc.updateGroup(ctx, id, in)
	}
	if err == nil {
		slog.Info("group saved", "by", username(ctx), "group", group.Name, "permissions", group.Permissions, "all_servers", group.AllServers)
	}
	write(w, r, status, group, err)
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	group, err := h.svc.Group(ctx, r.PathValue("id"))
	switch {
	case err != nil:
	case group.Builtin:
		err = errBuiltin
	case !From(ctx).Covers(group.grants()):
		err = errEscalate
	default:
		err = h.svc.deleteGroup(ctx, group.ID)
	}
	if err == nil {
		slog.Info("group deleted", "by", username(ctx), "group", group.Name)
	}
	write(w, r, http.StatusNoContent, nil, err)
}

// userView is a user with the IDs of their groups.
type userView struct {
	auth.Account
	Groups []string `json:"groups"`
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, _, err := h.state(r.Context())
	write(w, r, http.StatusOK, users, err)
}

// state returns all users with their groups, and all groups.
func (h *Handler) state(ctx context.Context) ([]userView, []Group, error) {
	accounts, err := h.users.Accounts(ctx)
	if err != nil {
		return nil, nil, err
	}
	groups, err := h.svc.Groups(ctx)
	users := make([]userView, len(accounts))
	for i, a := range accounts {
		users[i] = userView{Account: a, Groups: []string{}}
		for _, g := range groups {
			if slices.Contains(g.Members, a.ID) {
				users[i].Groups = append(users[i].Groups, g.ID)
			}
		}
	}
	return users, groups, err
}

// invite adds a user to the chosen groups and returns the link to set a password.
func (h *Handler) invite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string   `json:"username"`
		Groups   []string `json:"groups"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	_, groups, err := h.state(ctx)
	if err == nil {
		err = grantable(From(ctx), groups, nil, req.Groups)
	}
	var user auth.User
	var link auth.SetupLink
	if err == nil {
		user, link, err = h.users.Invite(ctx, req.Username)
	}
	if err == nil {
		err = h.svc.setGroups(ctx, user.ID, req.Groups)
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	slog.Info("user invited", "by", username(ctx), "user", user.Username, "groups", req.Groups)
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"user": user, "setupLink": link})
}

// updateUser changes the groups of a user and whether the user is disabled.
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Disabled bool     `json:"disabled"`
		Groups   []string `json:"groups"`
	}
	if err := httpapi.ReadJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	target, groups, err := h.manageable(r)
	if err == nil {
		err = grantable(From(ctx), groups, target.Groups, req.Groups)
	}
	if err == nil && req.Disabled && isSelf(ctx, target.ID) {
		err = errSelf
	}
	if err == nil && (req.Disabled || !slices.Contains(req.Groups, AdminGroup)) {
		err = h.keepAdmin(ctx, target.ID)
	}
	if err == nil {
		err = h.svc.setGroups(ctx, target.ID, req.Groups)
	}
	if err == nil && req.Disabled != target.Disabled {
		err = h.users.SetDisabled(ctx, target.ID, req.Disabled)
	}
	if err == nil {
		slog.Info("user changed", "by", username(ctx), "user", target.Username, "groups", req.Groups, "disabled", req.Disabled)
	}
	write(w, r, http.StatusNoContent, nil, err)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	target, _, err := h.manageable(r)
	if err == nil && isSelf(ctx, target.ID) {
		err = errSelf
	}
	if err == nil {
		err = h.keepAdmin(ctx, target.ID)
	}
	if err == nil {
		err = h.users.Delete(ctx, target.ID)
	}
	if err == nil {
		slog.Info("user deleted", "by", username(ctx), "user", target.Username)
	}
	write(w, r, http.StatusNoContent, nil, err)
}

// setupLink creates a new link for a user to set a password, e.g. after forgetting it.
func (h *Handler) setupLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	target, _, err := h.manageable(r)
	var link auth.SetupLink
	if err == nil {
		link, err = h.users.NewSetupLink(ctx, target.ID)
	}
	if err == nil {
		slog.Info("setup link created", "by", username(ctx), "user", target.Username)
	}
	write(w, r, http.StatusOK, link, err)
}

// manageable returns the user of the path if the signed-in user has every permission of
// that user, and all groups.
func (h *Handler) manageable(r *http.Request) (userView, []Group, error) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	users, groups, err := h.state(r.Context())
	if err != nil {
		return userView{}, nil, err
	}
	i := slices.IndexFunc(users, func(u userView) bool { return u.ID == id })
	if i < 0 {
		return userView{}, nil, httpapi.Errorf(http.StatusNotFound, "User not found.")
	}
	target, err := h.svc.Grants(r.Context(), id)
	if err == nil && !From(r.Context()).Covers(target) {
		err = errStronger
	}
	return users[i], groups, err
}

// grantable checks that the groups exist and that those added or removed give no more
// than the actor has.
func grantable(actor Grants, groups []Group, from, to []string) error {
	for _, id := range to {
		if !slices.ContainsFunc(groups, func(g Group) bool { return g.ID == id }) {
			return errGroupNotFound
		}
	}
	for _, g := range groups {
		if slices.Contains(from, g.ID) != slices.Contains(to, g.ID) && !actor.Covers(g.grants()) {
			return errEscalate
		}
	}
	return nil
}

// keepAdmin fails if no enabled administrator would remain without the user.
func (h *Handler) keepAdmin(ctx context.Context, userID int64) error {
	users, _, err := h.state(ctx)
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.ID != userID && !u.Disabled && slices.Contains(u.Groups, AdminGroup) {
			return nil
		}
	}
	if i := slices.IndexFunc(users, func(u userView) bool { return u.ID == userID }); i < 0 || !slices.Contains(users[i].Groups, AdminGroup) {
		return nil // not an administrator, nothing changes
	}
	return errLastAdmin
}

func isSelf(ctx context.Context, userID int64) bool {
	user, ok := auth.UserFrom(ctx)
	return ok && user.ID == userID
}

func username(ctx context.Context) string {
	user, _ := auth.UserFrom(ctx)
	return user.Username
}

func write(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, status, v)
}
