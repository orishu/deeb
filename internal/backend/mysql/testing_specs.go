package mysql

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
        - name: MYSQL_DATADIR
          value: "/var/lib/mysql/active"
        image: percona/percona-server:8.0
        imagePullPolicy: IfNotPresent
        command:
        - sh
        - -c
        - |
          echo "MySQL container starting - giving time for any BLOCKED file coordination..."
          # Give a small window for BLOCKED file to be created if snapshot restore is starting
          sleep 2
          
          echo "Checking for BLOCKED file..."
          while [ -f /var/lib/mysql/BLOCKED ]; do
            echo "MySQL startup blocked - waiting for snapshot restore to complete..."
            sleep 0.5
          done
          echo "No BLOCKED file found - proceeding with MySQL startup..."
          
          # Start MySQL normally
          exec /docker-entrypoint.sh mysqld
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
        - mountPath: /etc/my.cnf.d/repl.cnf
          name: configurations
          subPath: repl.cnf
      dnsPolicy: ClusterFirst
      initContainers:
      - command:
        - sh
        - -c
        - |
          echo "Cleaning up and setting permissions for shared volume..."
          rm -rf /var/lib/mysql/lost+found
          mkdir -p /var/lib/mysql/active
          chown -R 1001:1001 /var/lib/mysql/active
          chmod -R 755 /var/lib/mysql/active
          chown 1001:1001 /var/lib/mysql
          chmod 755 /var/lib/mysql
          echo "Permissions set: $(ls -la /var/lib/mysql/)"
        image: busybox:1.25.0
        imagePullPolicy: IfNotPresent
        name: setup-permissions
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
