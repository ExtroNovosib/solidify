package fixture

type Broad interface {
	DeleteExpired()
	GetArtifact()
	GetProtectedPayload()
	InsertArtifact()
	ListArtifactsByRun()
}
type Narrow interface{ InsertArtifact() }
type Runner struct{ store Narrow }

func NewRunner(artifactStores ...Broad) *Runner {
	var alias Broad
	if len(artifactStores) > 0 {
		alias = artifactStores[0]
	}
	return &Runner{alias}
}
