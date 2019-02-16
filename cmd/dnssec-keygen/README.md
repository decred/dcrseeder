
dnssec-keygen
=============

This is a CLI tool that generates the zone signing key (ZSK) and key signing key (KSK) which are required to run a DNSSEC enabled zone.  The generated key files are saved in the default app directory (`~/.dcrseeder`), their naming is identical to those of existing DNSSEC tools such as [ldns](https://www.nlnetlabs.nl/projects/ldns/about/).

## Building

```bash
$ go build ./cmd/...
```



## Usage

#### Generate keys

```bash
 ./dnssec-keygen -H mainnet-seed.example.com
```

```bash
 2019/02/26 18:39:58 DNSSEC signing keys not loaded
 2019/02/26 18:39:58 Generating ZSK
 2019/02/26 18:39:58 Private key generated, keytag: Kmainnet-seed.example.com.+010+51483
 2019/02/26 18:39:58 Created key /home/peter/.dcrseeder/Kmainnet-seed.example.com.+010+51483.private
 2019/02/26 18:39:58 Created key /home/peter/.dcrseeder/Kmainnet-seed.example.com.+010+51483.key
 2019/02/26 18:39:58 Generating KSK
 2019/02/26 18:39:58 Private key generated, keytag: Kmainnet-seed.example.com.+010+13753
 2019/02/26 18:39:58 Created key /home/peter/.dcrseeder/Kmainnet-seed.example.com.+010+13753.private
 2019/02/26 18:39:58 Created key /home/peter/.dcrseeder/Kmainnet-seed.example.com.+010+13753.key
 2019/02/26 18:39:58 Created key /home/peter/.dcrseeder/Kmainnet-seed.example.com.+010+13753.ds
...

```

This command saved the key files in `~/.dcrseedder/Kmainnet-seed.example.com.+010+51483` (ZSK) and `~/.dcrseeder/Kmainnet-seed.example.com.+010+13753` (KSK).

#### Configure ZSK/KSK

Configure the keys in `~/.dcrseeder/dcrseeder.conf` by adding the `zsk` and `ksk` configuration options (copy/paste the actual output from the `dnssec-keygen` tool above):

```
zsk=Kmainnet-seed.example.com.+010+51483
ksk=Kmainnet-seed.example.com.+010+13753
```

#### Configure DS record

In order to enable DNSSEC on the seeder zone (`mainnet-seed.example.com` in this example), you'll need to configure the delegation signer (`DS`) record in the parent zone, `example.com` in this example.  This involves adding an RR to the name server running the parent zone so the exact method will vary depending on the server software.  The content of the RR has been generated in the `*.ds` file along with the keys, `~/.dcrseeder/Kmainnet-seed.example.com.+010+13753.ds` in this example.


The content of the file looks like this:
```
mainnet-seed.example.com.       3600    IN      DS      13753 10 2 4379CEFDF1BDDEA9114682024A0DA20E353D7A77FDB1957CEB55D1B1E426B849
```

Once the `DS` record is added and the parent zone is reloaded, `dcrseeder` is ready to start.

## Testing

The [Verisign DNSSEC Analyzer](https://dnssec-analyzer.verisignlabs.com/) should validate your seed zone fully.


[DNSViz](http://dnsviz.net/) is a useful tool for debugging any issues in the DNSSEC configuration.

## More information

[How DNSSEC Works](https://www.cloudflare.com/dns/dnssec/how-dnssec-works/)
