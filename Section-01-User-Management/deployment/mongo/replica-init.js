// This administrative one-shot is separate from the restricted runtime account.
const admin = db.getSiblingDB('admin');
let initialized = false;
try {
  const state = admin.runCommand({replSetGetStatus: 1});
  initialized = state.ok === 1;
  if (!initialized && state.code !== 94) throw new Error('replica status failed');
} catch (error) {
  if (error.code !== 94) throw error;
}
if (!initialized) {
  const result = admin.runCommand({replSetInitiate: {_id: 'rs0', members: [{_id: 0, host: 'user-mongo:27017'}]}});
  if (result.ok !== 1) throw new Error('replica initialization failed');
}
let primary = false;
for (let attempt = 0; attempt < 120; attempt++) {
  if (admin.runCommand({hello: 1}).isWritablePrimary) { primary = true; break; }
  sleep(500);
}
if (!primary) throw new Error('primary election timed out');
const identity = db.getSiblingDB('identity');
const role = {
  role: 'user_runtime',
  privileges: [
    {resource: {db: 'identity', collection: 'users'}, actions: ['find', 'insert', 'update', 'remove']},
    {resource: {db: 'identity', collection: 'user_audits'}, actions: ['insert']}
  ], roles: []
};
if (identity.getRole(role.role)) identity.updateRole(role.role, {privileges: role.privileges, roles: []});
else identity.createRole(role);
identity.updateUser('identity', {roles: [{role: role.role, db: 'identity'}]});
print('Replica set ready; runtime audit collection is insert-only.');
