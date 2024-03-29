package testing

// ConfigMapSpec is a common MySQL config used in testing
var ConfigMapSpec string = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: t1-deeb-configuration
  namespace: test
data:
  repl.cnf: |-
    [mysqld]
    datadir=/var/lib/mysql/active
    gtid_mode=ON
    enforce_gtid_consistency=ON
`

// SSHSecretSpec is a common RSA key used in testing
var SSHSecretSpec string = `
apiVersion: v1
data:
  id_rsa: LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KYjNCbGJuTnphQzFyWlhrdGRqRUFBQUFBQkc1dmJtVUFBQUFFYm05dVpRQUFBQUFBQUFBQkFBQUJsd0FBQUFkemMyZ3RjbgpOaEFBQUFBd0VBQVFBQUFZRUF3dlI4amZ3VnpDM00rb01iZ0VZSkFTRGRseHRWWDZSMDZtU2doc2dVTDB2cllYclNIM3NYCkpBZzJNWHhTT0ZNcE9EMkp5SUNkQWpDeFJ3R3huQjU5dHRaVTQxZ2g4b3MrbWZLU3RZWXBzbWUwV1Jia3pPY1ZNVWVObkYKbkVwUHJzWXlqNERFN2h4VjJ0YXNwZTNYZlZuMW9ZeFUycmNIa1EvL0tScEw2ZDZHTHYvM2s5UkdpaGtycmEzMTQ1Q1BTYQpiM29Dditrd1JRdmxzMWRGYXR2UjNKZzZSL3UwL2hKUnpyR3Q1RWtDNHh6QndLYnEzdnB3cnhrNE1VK1NhUkkyaDFtNEhoClNHVmJFT0tVQ3ExSHVsT2JtR1BmMm1CSlpmekUrbE5nRXV1WDJYUmVlNnljc25Vb3NYUGVXOVVXMDVkL210UWZURytoVmwKL3RhUTBKbnNQY0dLdEY4TVhpS2FuSHVEeTUySjlYZmFVLy9kN2NsbjRoTVZ5L1k2RmFRQk9xcE0zVEsxSWUwU1FneVdZcQpYOENlaExnRlpuc2hNMkd5cHk0SExDN0NueUFuZHZERGNrVTBoOFFtUHo3b3Y5T0NXZWVCYUJEVVc5R2s4V0s2bVRDM3VBCk5yeE1jQlpVaFNuckR4V21SbXhidUtabm9nL0swQjBwNVg3Y0llQnpBQUFGaUJGdnk4TVJiOHZEQUFBQUIzTnphQzF5YzIKRUFBQUdCQU1MMGZJMzhGY3d0elBxREc0QkdDUUVnM1pjYlZWK2tkT3Brb0liSUZDOUw2MkY2MGg5N0Z5UUlOakY4VWpoVApLVGc5aWNpQW5RSXdzVWNCc1p3ZWZiYldWT05ZSWZLTFBwbnlrcldHS2JKbnRGa1c1TXpuRlRGSGpaeFp4S1Q2N0dNbytBCnhPNGNWZHJXcktYdDEzMVo5YUdNVk5xM0I1RVAveWthUytuZWhpNy85NVBVUm9vWks2MnQ5ZU9RajBtbTk2QXIvcE1FVUwKNWJOWFJXcmIwZHlZT2tmN3RQNFNVYzZ4cmVSSkF1TWN3Y0NtNnQ3NmNLOFpPREZQa21rU05vZFp1QjRVaGxXeERpbEFxdApSN3BUbTVoajM5cGdTV1g4eFBwVFlCTHJsOWwwWG51c25MSjFLTEZ6M2x2VkZ0T1hmNXJVSDB4dm9WWmY3V2tOQ1o3RDNCCmlyUmZERjRpbXB4N2c4dWRpZlYzMmxQLzNlM0paK0lURmN2Mk9oV2tBVHFxVE4weXRTSHRFa0lNbG1LbC9Bbm9TNEJXWjcKSVROaHNxY3VCeXd1d3A4Z0ozYnd3M0pGTklmRUpqOCs2TC9UZ2xubmdXZ1ExRnZScFBGaXVwa3d0N2dEYThUSEFXVklVcAo2dzhWcGtac1c3aW1aNklQeXRBZEtlViszQ0hnY3dBQUFBTUJBQUVBQUFHQkFLenUzb1c4TlVHMjV2cll6YzVOVWJOMGlkCnQrWFk3SGZRWm1XSmIyYUNGRVFQbHBUM2FwWTIrTThUV1lSLzY2bGZmVGJxTXlveFBNU1pUcEJibXN1bXN6V0gyS01pTEsKTGErMW96bnVWcEp3dDJQSGtSSEpjZDBTMGFUOVpCZk1sVitvZWMvQk1UZzN6cHJLQkxpRGtqVVdZSjYwTlAxQ0J6aGkzWgpxN2s2c09DRUlnTTU2NUNZbjB3aTRka0k1SEc2OERGZWxTV29VRTlxN05IVUNhMlYvS2tQaEZhTmx2T3E0VW9tRzcraG1uCjZwNlA4Z3YxTDN2QzdUdWdrQWt3SXkwK3piNG1NcTd6RUlWN0pSdWFNeTVDNnhaMUM4NklHeDRwM3dIRkVFeEtzczhwUVAKL1UvUTFiNHp3SFA5dk0wbDMyRDZqc1Z3ZnVuTlpneVQvK2IzRkxiWE5pM0dqckIxcnZiajdQVU10ZzR3Y2NRZUY2NVpacwpyNUxjWmpILzdDNEZwQ3lIY0RxejhzNGE5Y3V2WkpHd3BOaElYRmFaeEU3VlZGR2o0WFVnWmpyVUZxbUR6QmZoVk5YUHRqClFVY0xvaDVDZUpQN3FRekxtZnNaZ0d4UXlrbXJLOGNjaklPVlVVUW1EL1Jpd0RWcHBPTERHQkRCVmgwQUI5OG5yaEFRQUEKQU1CdVNpMDFadnFzcm44Y2VnZHRIT3pWYnF4RVJsY08rYU9YdkZ3V010K1dWYXhxUndsU2pYbWY2RFhmcUFXS2p3OVBKZQpMU0ZUeU9qSVNrR0FDNFJONXhOYkQrNnNKYngwTmVuVTE2ekEwRHp4UGk1ckR2aFZBQ09MMXlEa0lTSXRuUkRKUi9ySEpQCkY0dFJKdkpjTXpyWmpDMjFIaU5Ub0pxNms3TG5ja0E1VForWWNLUUFkSktqT3VTbnNRbGdPd1lvUHZ4Rmd6NW41eEVIT3oKWUFiVkU0UVZURmlkWFVDT2NZVnAwMW1IeXpkZ3lteWM3SEZXbjRZb3o2RlNkRHNSRUFBQURCQU8rditRZ2RJV2Fwck0wYQpsQldKU1NjWkRORTZ0eDFRQ1BuaUtKUFJUSW1qWnBwNUlNemYvY2dMYzRscHNNVFl6ZkNzLzcyc2I5ZnA5UnZwNVBQLzFCCjl4MFIzUHdSWEJSWG9vMFplRk9QY29UdXo1OHcwSTZoUmFLL3JTZVpDT1MxMXJjV0pHVmZabjlOc3UvLzVleHhHVTQzSHEKZWtzWG50YzdKb1hkcm9NanVqNkYyRW5iUWdMV1ovejVGaVdPRk55ZkRRdHJ3N0RYd1p4MDVjSmFQQzg3MENyemhlOE5tUgo2aldST1ZvbFNGYXJwcHBDc0M2ejBKM3Q2S2ZzYndNd0FBQU1FQTBEa21mTVlHcWk4S3FMcXNWV0xlWWZRRnpDQXFYT25XCm1LRld0ZzVOSGxKT3BDeXlQUUpKUTE0QWhCenk3dDJVdmpGeWpvYUFocnQ0bldTY0pXdnJtNWx3TVo1ZE9WdUhBbmE2NmEKeitQTTdyL2M1aUxGTEs1WGNtVXgzenhIWVBrUEwvN3IxL0RUUm1EN0ZMMm13SnBIZDh4RklzR01ZblF0WGdJRkFSOTFtUwpod09LVVBzM3J5bEVQM3FkRm91b0ZxMC9pWXhyYTFqRTFQK0NRaDI1TDl0LzJ4RUFVN1FHQkx0Yys5WVZXZVp2eUovUS9FCjZoUjI0eC91TW83QTdCQUFBQURtOXlhVUJLWlc1dWVYTXRUVUpRQVFJREJBPT0KLS0tLS1FTkQgT1BFTlNTSCBQUklWQVRFIEtFWS0tLS0tCg==
  id_rsa.pub: c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCZ1FEQzlIeU4vQlhNTGN6Nmd4dUFSZ2tCSU4yWEcxVmZwSFRxWktDR3lCUXZTK3RoZXRJZmV4Y2tDRFl4ZkZJNFV5azRQWW5JZ0owQ01MRkhBYkdjSG4yMjFsVGpXQ0h5aXo2WjhwSzFoaW15WjdSWkZ1VE01eFV4UjQyY1djU2srdXhqS1BnTVR1SEZYYTFxeWw3ZGQ5V2ZXaGpGVGF0d2VSRC84cEdrdnAzb1l1Ly9lVDFFYUtHU3V0cmZYamtJOUpwdmVnSy82VEJGQytXelYwVnEyOUhjbURwSCs3VCtFbEhPc2Eza1NRTGpITUhBcHVyZStuQ3ZHVGd4VDVKcEVqYUhXYmdlRklaVnNRNHBRS3JVZTZVNXVZWTkvYVlFbGwvTVQ2VTJBUzY1ZlpkRjU3ckp5eWRTaXhjOTViMVJiVGwzK2ExQjlNYjZGV1grMXBEUW1ldzl3WXEwWHd4ZUlwcWNlNFBMblluMWQ5cFQvOTN0eVdmaUV4WEw5am9WcEFFNnFremRNclVoN1JKQ0RKWmlwZndKNkV1QVZtZXlFelliS25MZ2NzTHNLZklDZDI4TU55UlRTSHhDWS9QdWkvMDRKWjU0Rm9FTlJiMGFUeFlycVpNTGU0QTJ2RXh3RmxTRktlc1BGYVpHYkZ1NHBtZWlEOHJRSFNubGZ0d2g0SE09IG9yaUBKZW5ueXMtTUJQCg==
kind: Secret
metadata:
  name: my-ssh-key
  namespace: test
type: Opaque
`
