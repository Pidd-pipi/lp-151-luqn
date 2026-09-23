package model

func AllModels() []any {
	return []any{
		&UserIdentity{},
		&Tag{},
		&PostTag{},
		&Post{},
		&Comment{},
		&Like{},
		&SensitiveWord{},
		&ReviewQueue{},
	}
}
