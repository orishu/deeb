package mysql

import (
	"context"
	"testing"

	"github.com/orishu/deeb/internal/lib"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

var configMapSpec string = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: t1-deeb-configuration
  namespace: test
data:
  repl.cnf: |-
    [mysqld]
    gtid_mode=ON
    enforce_gtid_consistency=ON
`

var statefulSetSpec string = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  annotations:
    unit-testing: test_name
  labels:
    app.kubernetes.io/instance: t1
    app.kubernetes.io/name: deeb
  name: t1-deeb
  namespace: test
spec:
  podManagementPolicy: OrderedReady
  replicas: 1
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app.kubernetes.io/instance: t1
      app.kubernetes.io/name: deeb
  serviceName: t1-deeb
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: t1
        app.kubernetes.io/name: deeb
    spec:
      containers:
      - env:
        - name: MYSQL_ALLOW_EMPTY_PASSWORD
          value: "true"
        - name: MYSQL_USER
        - name: MYSQL_DATABASE
          value: testdb
        image: percona:ps-8.0
        imagePullPolicy: IfNotPresent
        livenessProbe:
          exec:
            command:
            - mysqladmin
            - ping
          failureThreshold: 3
          initialDelaySeconds: 30
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 5
        name: t1-deeb
        ports:
        - containerPort: 3306
          name: mysql
          protocol: TCP
        readinessProbe:
          exec:
            command:
            - mysqladmin
            - ping
          failureThreshold: 3
          initialDelaySeconds: 5
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: data
        - mountPath: /etc/my.cnf.d/repl.cnf
          name: configurations
          subPath: repl.cnf
      - image: sidecar:latest
        imagePullPolicy: IfNotPresent
        name: t1-deeb-sidecar
        ports:
        - containerPort: 22
          name: ssh
          protocol: TCP
        resources:
          requests:
            cpu: 50m
            memory: 128Mi
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: data
        - mountPath: /var/secrets
          name: ssh-keys
      dnsPolicy: ClusterFirst
      initContainers:
      - command:
        - rm
        - -fr
        - /var/lib/mysql/lost+found
        image: busybox:1.25.0
        imagePullPolicy: IfNotPresent
        name: remove-lost-found
        resources: {}
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: data
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
      volumes:
      - configMap:
          defaultMode: 420
          name: t1-deeb-configuration
        name: configurations
      - name: ssh-keys
        secret:
          defaultMode: 420
          items:
          - key: id_rsa
            mode: 256
            path: id_rsa
          - key: id_rsa.pub
            path: id_rsa.pub
          secretName: my-ssh-key
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      creationTimestamp: null
      labels:
        app.kubernetes.io/instance: t1
        app.kubernetes.io/name: deeb
      name: data
    spec:
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 100Mi
      volumeMode: Filesystem
`

func Test_basic_mysql_access(t *testing.T) {
	ctx := context.Background()

	logger := lib.NewDevelopmentLogger()
	kubeHelper, err := lib.NewKubeHelper("test", logger)
	require.NoError(t, err)

	err = kubeHelper.EnsureConfigMap(ctx, "t1-deeb-configuration", configMapSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureStatefulSet(ctx, "t1-deeb", statefulSetSpec)
	require.NoError(t, err)

	podName := "t1-deeb-0"
	err = kubeHelper.WaitForPodToBeReady(ctx, podName, 30)
	require.NoError(t, err)

	ports, err := freeport.GetFreePorts(1)
	require.NoError(t, err)
	mysqlPort := ports[0]

	portForwardCloser, err := kubeHelper.PortForward(podName, mysqlPort, 3306)
	require.NoError(t, err)
	defer portForwardCloser()

	/*
		fmt.Println("Sleeping...")
		time.Sleep(5 * time.Minute)
		fmt.Println("Finished sleeping.")
	*/

	b, _ := New(Params{EntriesToRetain: 5, MysqlPort: mysqlPort}, logger)
	err = b.Start(ctx)
	defer b.Stop(ctx)
	require.NoError(t, err)

	/*
		dir, err := ioutil.TempDir("./testdb", fmt.Sprintf("%s-*", t.Name()))
		defer os.RemoveAll(dir)

		require.NoError(t, err)
		b, st := New(Params{DBDir: dir, EntriesToRetain: 5}, lib.NewDevelopmentLogger())
		ctx := context.Background()
		err = b.Start(ctx)
		defer b.Stop(ctx)
		require.NoError(t, err)

		err = b.AppendEntries(ctx, []raftpb.Entry{
			{Index: 1, Term: 1, Type: raftpb.EntryNormal, Data: []byte("hello")},
			{Index: 2, Term: 1, Type: raftpb.EntryNormal, Data: []byte("world")},
			{Index: 3, Term: 2, Type: raftpb.EntryNormal, Data: []byte("hi")},
			{Index: 4, Term: 2, Type: raftpb.EntryNormal, Data: []byte("there")},
			{Index: 5, Term: 2, Type: raftpb.EntryNormal, Data: []byte("fifth")},
		})
		require.NoError(t, err)

		err = b.AppendEntries(ctx, []raftpb.Entry{
			{Index: 4, Term: 2, Type: raftpb.EntryNormal, Data: []byte("there2")},
		})
		_, err = st.Entries(5, 5, 1)
		require.Error(t, err)

		err = b.SaveHardState(ctx, &raftpb.HardState{Term: 1, Vote: 12, Commit: 100})
		require.NoError(t, err)
		err = b.SaveHardState(ctx, &raftpb.HardState{Term: 2, Vote: 12, Commit: 101})
		require.NoError(t, err)

		term, err := st.Term(2)
		require.NoError(t, err)
		require.Equal(t, uint64(1), term)

		entries, err := st.Entries(2, 4, 10)
		require.NoError(t, err)
		require.Equal(t,
			[]raftpb.Entry{
				{Index: 2, Term: 1, Type: raftpb.EntryNormal, Data: []byte("world")},
				{Index: 3, Term: 2, Type: raftpb.EntryNormal, Data: []byte("hi")},
			},
			entries,
		)

		minIdx, err := st.FirstIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(1), minIdx)
		maxIdx, err := st.LastIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(4), maxIdx)

		err = b.SaveConfState(ctx, &raftpb.ConfState{Nodes: []uint64{3, 4}, Learners: []uint64{5}})
		require.NoError(t, err)

		err = b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 3, Addr: "localhost", Port: "10000"})
		require.NoError(t, err)
		err = b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 4, Addr: "localhost", Port: "10001"})
		require.NoError(t, err)
		peerInfos, err := b.LoadPeers(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, len(peerInfos))
		require.Equal(t, backend.PeerInfo{NodeID: 3, Addr: "localhost", Port: "10000"}, peerInfos[0])
		require.Equal(t, backend.PeerInfo{NodeID: 4, Addr: "localhost", Port: "10001"}, peerInfos[1])

		err = b.RemovePeer(ctx, 3)
		require.NoError(t, err)

		hs, cs, err := st.InitialState()
		require.NoError(t, err)
		require.Equal(t, raftpb.ConfState{Nodes: []uint64{3, 4}, Learners: []uint64{5}}, cs)
		require.Equal(t, raftpb.HardState{Term: 2, Vote: 12, Commit: 101}, hs)

		err = b.SaveApplied(ctx, 10, 123)
		require.NoError(t, err)

		snap, err := st.Snapshot()
		require.NoError(t, err)
		require.Equal(t, uint64(10), snap.Metadata.Term)
		require.Equal(t, uint64(123), snap.Metadata.Index)
		tarFile, err := os.OpenFile(dir+"/testtar.tar", os.O_WRONLY|os.O_CREATE, 0644)
		_, err = io.Copy(tarFile, bytes.NewReader(snap.Data))
		require.NoError(t, err)
		err = tarFile.Close()
		require.NoError(t, err)

		err = b.RemovePeer(ctx, 4)
		require.NoError(t, err)

		tarFile, err = os.Open(dir + "/testtar.tar")
		require.NoError(t, err)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, tarFile)
		require.NoError(t, err)
		tarFile.Close()

		// Remember the current conf state
		_, cs, err = st.InitialState()
		require.NoError(t, err)

		// Override conf state before restoring from snapshot
		err = b.SaveConfState(ctx, &raftpb.ConfState{Nodes: []uint64{13, 14}, Learners: []uint64{15}})
		require.NoError(t, err)

		// Restore from snapshot, use the remembered conf state as metadata
		snapMeta := raftpb.SnapshotMetadata{Term: 30, Index: 300, ConfState: cs}
		snap2 := raftpb.Snapshot{Data: buf.Bytes(), Metadata: snapMeta}
		err = b.ApplySnapshot(ctx, snap2)
		require.NoError(t, err)

		// Check that the overriden conf state is back
		_, cs, err = st.InitialState()
		require.NoError(t, err)
		require.Equal(t, raftpb.ConfState{Nodes: []uint64{3, 4}, Learners: []uint64{5}}, cs)

		err = b.ExecSQL(ctx, 1, 1, "CREATE TABLE table1 (col1 INTEGER, col2 INTEGER)")
		require.NoError(t, err)
		err = b.ExecSQL(ctx, 1, 2, "INSERT INTO table1 (col1, col2) VALUES (100, 200), (101, 201)")
		require.NoError(t, err)
		rows, err := b.QuerySQL(ctx, "SELECT col1, col2 FROM table1")
		require.NoError(t, err)
		require.True(t, rows.Next())
		var v1, v2 int
		err = rows.Scan(&v1, &v2)
		require.NoError(t, err)
		require.Equal(t, 100, v1)
		require.Equal(t, 200, v2)
		require.True(t, rows.Next())
		err = rows.Scan(&v1, &v2)
		require.NoError(t, err)
		require.Equal(t, 101, v1)
		require.Equal(t, 201, v2)
		require.False(t, rows.Next())

		// Append some more entries to see older ones deleted
		err = b.AppendEntries(ctx, []raftpb.Entry{
			{Index: 5, Term: 2, Type: raftpb.EntryNormal, Data: []byte("one")},
			{Index: 6, Term: 2, Type: raftpb.EntryNormal, Data: []byte("two")},
			{Index: 7, Term: 2, Type: raftpb.EntryNormal, Data: []byte("three")},
		})
		require.NoError(t, err)

		minIdx, err = st.FirstIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(3), minIdx)
		maxIdx, err = st.LastIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(7), maxIdx)
	*/
}
