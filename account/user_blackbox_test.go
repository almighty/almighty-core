package account_test

import (
	"os"
	"testing"

	"github.com/almighty/almighty-core/account"
	"github.com/almighty/almighty-core/gormsupport/cleaner"
	"github.com/almighty/almighty-core/gormtestsupport"
	"github.com/almighty/almighty-core/migration"
	"github.com/almighty/almighty-core/models"
	"github.com/almighty/almighty-core/resource"
	"github.com/almighty/almighty-core/workitem"

	"github.com/jinzhu/gorm"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
)

type userBlackBoxTest struct {
	gormtestsupport.DBTestSuite
	repo  account.UserRepository
	clean func()
	ctx   context.Context
}

func TestRunUserBlackBoxTest(t *testing.T) {
	suite.Run(t, &userBlackBoxTest{DBTestSuite: gormtestsupport.NewDBTestSuite("../config.yaml")})
}

// SetupSuite overrides the DBTestSuite's function but calls it before doing anything else
// The SetupSuite method will run before the tests in the suite are run.
// It sets up a database connection for all the tests in this suite without polluting global space.
func (s *userBlackBoxTest) SetupSuite() {
	s.DBTestSuite.SetupSuite()

	// Make sure the database is populated with the correct types (e.g. bug etc.)
	if _, c := os.LookupEnv(resource.Database); c != false {
		if err := models.Transactional(s.DB, func(tx *gorm.DB) error {
			s.ctx = migration.NewMigrationContext(context.Background())
			return migration.PopulateCommonTypes(s.ctx, tx, workitem.NewWorkItemTypeRepository(tx))
		}); err != nil {
			panic(err.Error())
		}
	}
}

func (s *userBlackBoxTest) SetupTest() {
	s.repo = account.NewUserRepository(s.DB)
	s.clean = cleaner.DeleteCreatedEntities(s.DB)
}

func (s *userBlackBoxTest) TearDownTest() {
	//s.clean()
}

func (s *userBlackBoxTest) TestOKToDelete() {
	t := s.T()
	resource.Require(t, resource.Database)

	// create 2 users, where the first one would be deleted.
	user := createAndLoadUser(s)
	createAndLoadUser(s)

	err := s.repo.Delete(s.ctx, user.ID)
	assert.Nil(s.T(), err)

	// lets see how many are present.
	users, err := s.repo.List(s.ctx)
	require.Nil(s.T(), err, "Could not list users")
	require.True(s.T(), len(users) > 0)

	for _, data := range users {
		// The user 'user' was deleted and rest were not deleted, hence we check
		// that none of the user objects returned include the one deleted.
		require.NotEqual(s.T(), user.ID.String(), data.ID.String())
	}
}

func (s *userBlackBoxTest) TestOKToLoad() {
	t := s.T()
	resource.Require(t, resource.Database)

	createAndLoadUser(s) // this function does the needful already
}

func (s *userBlackBoxTest) TestOKToSave() {
	t := s.T()
	resource.Require(t, resource.Database)

	user := createAndLoadUser(s)

	user.FullName = "newusernameTestUser"
	err := s.repo.Save(s.ctx, user)
	require.Nil(s.T(), err, "Could not update user")

	updatedUser, err := s.repo.Load(s.ctx, user.ID)
	require.Nil(s.T(), err, "Could not load user")
	require.Equal(s.T(), user.FullName, updatedUser.FullName)
	fields := user.ContextInformation
	require.Equal(s.T(), fields["last_visited"], "http://www.google.com")
}

func createAndLoadUser(s *userBlackBoxTest) *account.User {
	user := &account.User{
		ID:       uuid.NewV4(),
		Email:    "someuser@TestUser" + uuid.NewV4().String(),
		FullName: "someuserTestUser" + uuid.NewV4().String(),
		ImageURL: "someImageUrl" + uuid.NewV4().String(),
		Bio:      "somebio" + uuid.NewV4().String(),
		URL:      "someurl" + uuid.NewV4().String(),
		ContextInformation: workitem.Fields{
			"space":        uuid.NewV4(),
			"last_visited": "http://www.google.com",
		},
	}

	err := s.repo.Create(s.ctx, user)
	require.Nil(s.T(), err, "Could not create user")

	createdUser, err := s.repo.Load(s.ctx, user.ID)
	require.Nil(s.T(), err, "Could not load user")
	require.Equal(s.T(), user.Email, createdUser.Email)
	require.Equal(s.T(), user.ID, createdUser.ID)
	require.Equal(s.T(), user.ContextInformation, createdUser.ContextInformation)

	return createdUser
}
