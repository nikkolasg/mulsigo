# Usage & Design

Everything is contained in one binary. That just makes it easier for people to 
`go get -u github.com/nikkolasg/mulsigo` and then the command accordingly.

## Usage

Before any command from the users, the server must be launched by calling `mulsigo server`.

+ To setup a distributed key pair among one group. Note that it is possible to
  set up different key pairs for one group (think different pgp key for one
  person).
    Someone must initiate the key generation protocol by giving the usual
    information of a pgp key and a random password that is used to secure the
    connection between participants. this random password MUST ONLY be shared
    amongst the participants and MUST be one time use only:
    1. `mulsigo keygen-full -id "phrack rox" -e "phrack@rox.com" -p "randomPassword" -n 8`
    2. `mulsigo keygen-full -id "phrack" -p "randomPassword"`
    Or if users already have the gorup.toml setup:
    1. `mulsigo keygen -id "phrack rox" -e "phrack@rox.com" -p "randomPassword" -n 8`
    2. `mulsigo keygen -id "phrack" -p "randomPassword" -n 8`
    3. At the end of the process, every participant will see the pgp compatible 
       public key generated, including its ID.
    4. Every participants should have now have a group configuration, the
       private share and the public key stored under
       `~/.config/mulsigo/<GPG key ID>/`. Every participant is encouraged to keep a
       good protection hygiene on those folder and to do a backup.

+ To sign anything with a previous set distributed key, an initiator must run
    1. a. `mulsigo sign -id <gpg key id> <file>`
            =>  file will be sent directly, if not too big if passing through
            the relay
       b. `mulsigo sign -id <gpg key id> <url>`
            => url will be sent, peers will be asked if they want to download
            it. First over HTTP and HTTPS. Later on, more means.
       c. `mulsigo sign -id <gpg key id> --text "hello world"`
    2. Every other participant must run `mulsigo sign -id <gpg key id>`
    
    
    At the end of the process, everybody should get a `<file>.sig` signature
    file compatible with pgp.

+ To print the list of keys one user is currently "registered" for (think `gpg
  --list-secret-keys`):
   `mulsigo client list`

    
+ To make everything easy and secure for you, dear users, the server is actually
  nothing more than a proxy. For each connection, it reads a protobuf message
  looking like this:
  ```protobuf
  message OpaqueBlob {
      required bytes identifier
      optional bytes blob
  }
  ```
  The server reads the identifier for each message, and dispatch the paquet to
  each other connection sharing the same identifier. That's it. The `blob` field
  is actually encrypted by XXX.


######

## Relay
 
 - work over websocket
 - channel id is the path /channelid
 - multiple participant in channel possible 
 - actions: 
    - register to a channel
    - unregister to a channel
    - broadcast message to a channel

Security: 
    + drop messages or for some destinations only 
        -> same as bad network connections, and should be handled by the
        protocol
        -> detection by leader releasing the hash of the group.toml, so every peer can           check the final result if it's right or not.

IF asking relay to put "sender":
 - relay can lie,i.e. confuse sender / receiver.
    + if it's fake or different sender each time: nonces will be incorrect, so garbage
      will be detected.
    + if it's changing consistently between users: since they all use the same
      key, everything's going to decrypt correctly but the information sent will
      be exchanged. Since "sender" address is not authenticated nor used
      afterwards, the resulting toml group will be the same.
      -> relay can decide to exchange consistently up to some point where it
      reverts. -> Depending on the protocol, this might ABORT the protocol, so a
      DOS attack is possible. For example, for group.toml generation, 
    + In general, consider relay as an multiplexed insecure channel, so
      authentication is needed one layer above !

## group toml

### Solution 5

 - Every shares the same password
 - the channel ID is derived from the password with a proof that the peer knows
   the password itself with a random number. Look at Schnorr ZKP !!
   https://tools.ietf.org/html/draft-hao-schnorr-05

    -> useless since relay is untrusted vs NIZK verifier is honest...

### Solution 4

https://security.stackexchange.com/questions/10983/bcrypt-as-a-key-derivation-function
 - Every shares the same password
    - shared key by doing 
        func GenerateFromPassword(password []byte, cost int) ([]byte, error)
        OR ARGON2, by giving out salt. The exchange should not last long so
        attacker can't compute rainbow table.
            -> NOPE, cause everyone's supposed to share it.
        Solution: Send Nacl(packet), Salt(user), Nonce(packet) for each packet
    - nacl encryption of messages, nonce going from 1 to ...
      local nonces for each connections start from 1
      key is blake2b(shared_key, applicationContext)
 - ID is public, like mulsigo, tor, debian etc...
 Think of a channel like a public space where you only hear garbage noise if you
 don't know the password.
 - 1) Everyone connects on the channel by providing an username, generate
   private/public key pair,self signed.
 - Each time someone connects to the channel, his info is dispatched to everyone
   connected.  If info are invalid, broadcast a warning/leave message and leave.
   everyone leave upon warning message.
   FROM HERE : all messages are signed using this public private key pair, so
   attribution is possible !
 - There is a designated "leader", which knows the whole list of participant.
   Each 5sec, he broadcast a "Who's here" in the channel, everyone reply
   (including him). 
 - When the leader thinks everyone's here, he defines the group.toml, sends it
   to everyone. Everyone else check if they have the whole set of participants
   stored locally:
    -> no: broadcast the reduced group.toml, and in that case, everyone else
    ABORT
    -> yes: broadcast its hash.

ASSUMPTIONS: leader is trusted and knows the list of participants, password not leaked

IF peers are untrusted, they can:
 - leak the password 
    -> an attacker can see all messages, but nothing private. 
    -> can create multiple identities but is caught by the leader -> ABORT
 - create & broadcast multiple identities: this should be caught by the leader,
   which sends an "ABORT" message over the channel.
 - not sending any identity -> Leader should accept a treshold of missing peers
   when deciding to create group.
 - sending same username but different private/public key pair:
    -> ABORT ? DOS the whole thing. !! YES !! (conservative values)
    -> exclude both ? => can DOS the whole thing !! NO !!
 - not respond to "WHOSHERE" message: after a timeout * 3, drop the info from
   the list locally. Leader should accept a certain threshold.
 - Send a "OK" message instead of the leader
    -> this is bad... BAAAADDD
    => a leader is preferable as otherwise, it requires everyone to say OK on a
    set, but what happens when set are not the same... or one goes late.

IF peers are trusted:
 - relay can block or delay some messages for some destinations:
    -> need a way to check the final group toml is the same for everyone
    -> 

### Solution 3

https://security.stackexchange.com/questions/10983/bcrypt-as-a-key-derivation-function
 - Every shares the same password
    - shared key by doing 
        func GenerateFromPassword(password []byte, cost int) ([]byte, error)
    - nacl encryption of messages, nonce going from 1 to ...
      local nonces for each connections start from 1
      key is blake2b(shared_key, applicationContext)
 - ID is public, like mulsigo, tor, debian etc...
 Think of a channel like a public space where you only hear garbage noise if you
 don't know the password.
 - Everyone connects on the channel by providing an username, generate
   private/public key pair,self signed.
 - Each time someone connects to the channel, his info is dispatched to everyone
   connected.  If info are invalid, broadcast a warning/leave message and leave.
   everyone leave upon warning message.
 - Each user are being asked if [O]k to continue or [W]ait others, or
   [Q]uit.
    --> if OK, then new users are shown still but default action is Ok
    --> if W, then new users are shown, and action is asked again each time
    --> if Q, then user quits and sends a leave message.
 - OK messages are broadcasted to everyone and recorded. they are signed by the
   sender. For each
   participants, once it has as many OK as people recorded, it outputs a FINISH
   message also signed.
 - For each peer, if it is OK, and saw n FINISH messages,... 
 - Each peer sorts the info by username and create the group.toml

SECurity => it's not forward secure, but it's OK since, once the group.toml is
done, it's too late, and it's not private information. the rest IS forward secure. 

### Solution 1

 - Every shares the same password
 - ID of the channel is gen. from a _public_ username, a public email etc
    id = H(<string>) 
 - initiator sends its SPAKE2 handshake with the ID
 - each other participant sends its own SPAKE2 handshake with the ID
 - the relay all messages back to initiator for this phase
    => PB: WHO is initiator ?? Anybody can since ID is in clear
        Solution: 
            1. Needs a zero knowledge proof that the first node that sends this
              ID knows the password. K is = H(pwd). init. must prove that he
              owns K, by proving log_g g^k = log_h h^k !
 - initiator has now n SPAKE2 shared keys
 - initiator sends its own public key encrypted the derived key(nacl ? noise?)
    message needs to include the hash of the key || id to make sure it's fine
 - everyone sends back their own public key encrypted
 - initiator has now a group.toml which is broadcasted again encrypted (nacl ?)
    group.toml the list of Public keys

### Solution 2

 - Every shares the same password
 - ID of the channel is gen. from a _public_ username, a public email etc
    id = H(<string>) 
 - initiator sends its SPAKE2 handshake with the ID 
 - each other participant sends its own SPAKE2 handshake with the ID
 - Each participants has now own shared key with everyone else
 - Each participants know broadcast his newly created public key
    including: 
        + the hash of the key || id to make sure it's fine
        + self signature
 - Each participants collects everyone's public key, SORT IT => group.toml
 - Each participant signs the group.toml and broadcast
 - MANUAL verification:
    + how many keys
    + if "mine" is included
    + hash

## dist. key gen

 - every participants makes an ID for each other participants
    - p1 <-> p2 : id = H( PK1 || PK2 ) => order taken from group.toml
 - each participant start the DKG protocol. Each message is encrypted according
   the destination.
    - p1 -> p2 : 
        + ID = H( PK1 || PK2)
        + encryption done by 
            -> Nacl ?  no handshake using pub key, drawbacks ? 
            -> Noise ? handshake (eph. + static DH)
 - initiator proposes name, email and comment by sending the first message after
   handshake
 - non initiators are being asked if they are OK with creating this public key
   with name, email and comment.
 - signing takes place (dtss with another dkg round)

## general signing op.

 - every participants makes an ID for each other participants
    - p1 <-> p2 : id = H( PK1 || PK2 ) => order taken from group.toml
 - each participant start the DKG protocol. Each message is encrypted according
   the destination.
    - p1 -> p2 : 
        + ID = H( PK1 || PK2)
        + encryption done by 
            -> Nacl ?  no handshake using pub key, drawbacks ? 
            -> Noise ? handshake (eph. + static DH)
 - initiator sends 
    + link to download the "msg" to sign 
    + sends message directly if small
 - each one downloads & inspect, and say whether it's ok or not to sign
 - dtss takes place.

## Protocol

### Cryptography

Each participant willing to form a group to generate a distributed key must have
the same password. The password is then used to derive a strong high entropy key
to symmetrically encrypt the messages. 

### Server

The server is a proxy receiving `OpaqueBlob` messages, keeping those messages
for a short amount of time, let's say 5mn, and dispatching the messages to
every connection sending `OpaqueBlob` having the same "identifier". 




## OPEN ISSUES:
 - mechanism to have a private or random channel id 
    for the moment, id = H(pk1 | pk2)


## usage 2

mulsigo new private
musigo new dist
mulsigo sign 
musigo list --private --dist


## Relay API - Client

Given list of Identity with public key and potential address
    => deterministic way of using one or the other

Client.Router takes its own identity, and can send to an identity => realtime pair 
wise order to determine channel ID.
order 
