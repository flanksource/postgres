package pkg

type Extension interface {
	Install(p *Postgres) error
	IsInstalled(p *Postgres) error
	Health(p *Postgres) error
}
