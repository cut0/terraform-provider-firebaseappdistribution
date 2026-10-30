package provider

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"golang.org/x/oauth2"
	firebase "google.golang.org/api/firebase/v1beta1"
	"google.golang.org/api/firebaseappdistribution/v1"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

type client struct {
	appDistribution *firebaseappdistribution.Service
	firebase        *firebase.Service
	project         string
	projectNumbers  sync.Map
}

type clientConfig struct {
	project        string
	accessToken    string
	impersonate    string
	billingProject string
	userAgent      string
}

func newClient(ctx context.Context, config clientConfig) (*client, error) {
	options, err := credentialOptions(ctx, config)
	if err != nil {
		return nil, err
	}

	options = append(options,
		option.WithScopes(cloudPlatformScope),
		option.WithUserAgent(config.userAgent),
	)
	if config.billingProject != "" {
		options = append(options, option.WithQuotaProject(config.billingProject))
	}

	appDistribution, err := firebaseappdistribution.NewService(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("creating App Distribution client: %w", err)
	}

	firebaseService, err := firebase.NewService(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("creating Firebase Management client: %w", err)
	}

	return &client{
		appDistribution: appDistribution,
		firebase:        firebaseService,
		project:         config.project,
	}, nil
}

func (c *client) projectOrDefault(project string) string {
	if project != "" {
		return project
	}
	return c.project
}

func (c *client) projectNumber(ctx context.Context, project string) (string, error) {
	if isProjectNumber(project) {
		return project, nil
	}

	if cached, ok := c.projectNumbers.Load(project); ok {
		return cached.(string), nil
	}

	firebaseProject, err := c.firebase.Projects.Get(projectParent(project)).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("resolving project number of %q via the Firebase Management API: %w", project, err)
	}

	number := strconv.FormatInt(firebaseProject.ProjectNumber, 10)
	c.projectNumbers.Store(project, number)

	return number, nil
}

func credentialOptions(ctx context.Context, config clientConfig) ([]option.ClientOption, error) {
	var options []option.ClientOption

	if config.accessToken != "" {
		staticToken := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.accessToken})
		options = append(options, option.WithTokenSource(staticToken))
	}

	if config.impersonate == "" {
		return options, nil
	}

	tokenSource, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
		TargetPrincipal: config.impersonate,
		Scopes:          []string{cloudPlatformScope},
	}, options...)
	if err != nil {
		return nil, fmt.Errorf("configuring impersonation of %s: %w", config.impersonate, err)
	}

	return []option.ClientOption{option.WithTokenSource(tokenSource)}, nil
}
