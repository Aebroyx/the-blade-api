package pagination

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// JoinType represents the type of join
type JoinType string

const (
	LeftJoin  JoinType = "LEFT"
	InnerJoin JoinType = "INNER"
	RightJoin JoinType = "RIGHT"
)

// JoinConfig represents a join configuration
type JoinConfig struct {
	Table     string   // Table to join with
	Condition string   // Join condition
	Type      JoinType // Type of join (LEFT, INNER, RIGHT)
	Alias     string   // Optional alias for the joined table
}

// SelectField represents a field to select in the query
type SelectField struct {
	Field string // Field name or expression
	Alias string // Optional alias for the field
}

// QueryParams represents the common query parameters for pagination
type QueryParams struct {
	Page     int                    `json:"page" form:"page" binding:"min=1"`
	PageSize int                    `json:"pageSize" form:"pageSize" binding:"min=1,max=100"`
	Search   string                 `json:"search" form:"search"`
	Filters  map[string]interface{} `json:"filters" form:"filters" binding:"dive"`
	SortBy   string                 `json:"sortBy" form:"sortBy"`
	SortDesc bool                   `json:"sortDesc" form:"sortDesc"`
	Dates    map[string]DateRange   `json:"dates" form:"dates"`
}

// Custom binding for filters
func (qp *QueryParams) Bind(c *gin.Context) error {
	if err := c.ShouldBindQuery(qp); err != nil {
		return err
	}

	// Handle filters separately
	filters := make(map[string]interface{})
	for key, values := range c.Request.URL.Query() {
		if strings.HasPrefix(key, "filters[") && strings.HasSuffix(key, "]") {
			// Extract the field name from filters[field]
			field := key[8 : len(key)-1]
			if len(values) > 0 {
				filters[field] = values[0]
			}
		}
	}
	qp.Filters = filters

	return nil
}

// DateRange represents a date range filter
type DateRange struct {
	Start *time.Time `json:"start" form:"start"`
	End   *time.Time `json:"end" form:"end"`
}

// DateField represents a date field configuration
type DateField struct {
	Start string // Database column name for start date
	End   string // Database column name for end date
}

// PaginationConfig holds the configuration for pagination
type PaginationConfig struct {
	Model         interface{}            // The model to query (e.g., &models.Users{})
	BaseCondition map[string]interface{} // Base conditions (e.g., is_deleted = false)
	SearchFields  []string               // Fields to search in (e.g., ["name", "email", "username"])
	FilterFields  map[string]string      // Fields that can be filtered (e.g., {"role": "role"})
	DateFields    map[string]DateField   // Fields that are dates
	SortFields    []string               // Fields that can be sorted
	DefaultSort   string                 // Default sort field
	DefaultOrder  string                 // Default sort order ("ASC" or "DESC")
	Relations     []string               // Relations to preload
	Joins         []JoinConfig           // Joins to apply
	SelectFields  []SelectField          // Custom select fields
	GroupBy       []string               // Group by clauses
	Having        []string               // Having clauses
	Distinct      bool                   // Whether to use DISTINCT
	TableAlias    string                 // Alias for the main table
}

// PaginatedResponse represents the standard pagination response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

// Paginator handles the pagination logic
type Paginator struct {
	db *gorm.DB
}

// NewPaginator creates a new paginator instance
func NewPaginator(db *gorm.DB) *Paginator {
	return &Paginator{db: db}
}

// buildSelectClause builds the SELECT clause for the query
func (p *Paginator) buildSelectClause(config PaginationConfig) *gorm.DB {
	query := p.db.Model(config.Model)

	// Apply table alias if provided
	if config.TableAlias != "" {
		query = query.Table(fmt.Sprintf("%s AS %s",
			query.Statement.Table, config.TableAlias))
	}

	// Apply custom select fields if provided
	if len(config.SelectFields) > 0 {
		selectClause := make([]string, len(config.SelectFields))
		for i, field := range config.SelectFields {
			if field.Alias != "" {
				selectClause[i] = fmt.Sprintf("%s AS %s", field.Field, field.Alias)
			} else {
				selectClause[i] = field.Field
			}
		}
		query = query.Select(strings.Join(selectClause, ", "))
	}

	// Apply DISTINCT if needed
	if config.Distinct {
		query = query.Distinct()
	}

	return query
}

// buildJoinClause builds the JOIN clauses for the query
func (p *Paginator) buildJoinClause(query *gorm.DB, config PaginationConfig) *gorm.DB {
	for _, join := range config.Joins {
		joinClause := fmt.Sprintf("%s JOIN %s", join.Type, join.Table)
		if join.Alias != "" {
			joinClause += fmt.Sprintf(" AS %s", join.Alias)
		}
		joinClause += fmt.Sprintf(" ON %s", join.Condition)
		query = query.Joins(joinClause)
	}
	return query
}

// buildWhereClause builds the WHERE clause for the query
func (p *Paginator) buildWhereClause(query *gorm.DB, params QueryParams, config PaginationConfig) *gorm.DB {
	// Apply base conditions
	for field, value := range config.BaseCondition {
		query = query.Where(field+" = ?", value)
	}

	// Apply search if provided
	if params.Search != "" && len(config.SearchFields) > 0 {
		searchQuery := "%" + params.Search + "%"
		searchConditions := make([]string, len(config.SearchFields))
		searchArgs := make([]interface{}, len(config.SearchFields))

		for i, field := range config.SearchFields {
			searchConditions[i] = field + " ILIKE ?"
			searchArgs[i] = searchQuery
		}

		query = query.Where(strings.Join(searchConditions, " OR "), searchArgs...)
	}

	// Apply filters
	for field, value := range params.Filters {
		if dbField, ok := config.FilterFields[field]; ok && value != nil {
			query = query.Where(dbField+" = ?", value)
		}
	}

	// Apply date range filters if configured
	if len(config.DateFields) > 0 && len(params.Dates) > 0 {
		for field, dateRange := range params.Dates {
			if dbField, ok := config.DateFields[field]; ok {
				if dateRange.Start != nil {
					query = query.Where(dbField.Start+" >= ?", dateRange.Start)
				}
				if dateRange.End != nil {
					query = query.Where(dbField.End+" <= ?", dateRange.End)
				}
			}
		}
	}

	return query
}

// buildGroupByClause builds the GROUP BY and HAVING clauses
func (p *Paginator) buildGroupByClause(query *gorm.DB, config PaginationConfig) *gorm.DB {
	if len(config.GroupBy) > 0 {
		query = query.Group(strings.Join(config.GroupBy, ", "))
	}

	if len(config.Having) > 0 {
		query = query.Having(strings.Join(config.Having, " AND "))
	}

	return query
}

// Paginate executes the pagination query based on the provided parameters and config
func (p *Paginator) Paginate(params QueryParams, config PaginationConfig) (*PaginatedResponse, error) {
	// Set default values
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}
	if params.SortBy == "" {
		params.SortBy = config.DefaultSort
	}
	if config.DefaultOrder == "" {
		config.DefaultOrder = "DESC"
	}

	// Build the query step by step
	query := p.buildSelectClause(config)
	query = p.buildJoinClause(query, config)
	query = p.buildWhereClause(query, params, config)
	query = p.buildGroupByClause(query, config)

	// Get total count
	var total int64
	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Apply sorting
	if params.SortBy != "" {
		// Validate sort field
		isValidSort := false
		for _, field := range config.SortFields {
			if field == params.SortBy {
				isValidSort = true
				break
			}
		}

		if !isValidSort {
			params.SortBy = config.DefaultSort
		}

		sortOrder := "ASC"
		if params.SortDesc {
			sortOrder = "DESC"
		}
		query = query.Order(fmt.Sprintf("%s %s", params.SortBy, sortOrder))
	}

	// Apply relations if any
	if len(config.Relations) > 0 {
		query = query.Preload(strings.Join(config.Relations, " "))
	}

	// Apply pagination
	offset := (params.Page - 1) * params.PageSize
	query = query.Offset(offset).Limit(params.PageSize)

	// Execute query
	// Create a slice of the model type
	modelType := reflect.TypeOf(config.Model).Elem()
	sliceType := reflect.SliceOf(modelType)
	result := reflect.MakeSlice(sliceType, 0, 0).Interface()

	if err := query.Find(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}

	// Calculate total pages
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	return &PaginatedResponse{
		Data:       result,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}
