# Config 

Each distributed public key informations are stored in separate files under
"$HOME/.mulsigo/NAME.toml"

## Personal "local" information

Each local private key generated by mulsigo will be stored in 
"$HOME/.mulsigo/local/NAME/private.toml". NAME is the name given 
during the creation of the key pair. The public key will be stored in
"$HOME/.mulsigo/local/NAME/public.toml".

## Group information

+ Roster: The list of public keys of the share holders.
+ Threshold: the threshold t to use during computations and verifications 
+ Public key information: 
    - public coefficients of the distributed polynomials.  The first coefficient 
    is the distributed public key point.
    - public key, gpg encoded
+ Private key information: share of the longterm secret

The public information of the roster will be stored in 
"$HOME/.mulsigo/dist/NAME/public.toml" where 