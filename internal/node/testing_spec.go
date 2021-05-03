package node

var statefulSetSpec string = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: t1-deeb
  namespace: test
  labels:
    app.kubernetes.io/instance: t1
    app.kubernetes.io/name: deeb
spec:
  podManagementPolicy: OrderedReady
  replicas: 3
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
        - rm
        - -fr
        - /var/lib/mysql/lost+found
        image: busybox:1.25.0
        imagePullPolicy: IfNotPresent
        name: remove-lost-found
        resources:
          limits:
            cpu: 25m
            memory: 128Mi
          requests:
            cpu: 25m
            memory: 128Mi
        volumeMounts:
        - mountPath: /var/lib/mysql
          name: data
      restartPolicy: Always
      terminationGracePeriodSeconds: 30
      volumes:
      - name: configurations
        configMap:
          name: t1-deeb-configuration
      - name: ssh-keys
        secret:
          items:
          - key: id_rsa
            mode: 0400
            path: id_rsa
          - key: id_rsa.pub
            path: id_rsa.pub
          secretName: my-ssh-key
  volumeClaimTemplates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
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
