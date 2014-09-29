Configuration
==============

Momonga uses TOML syntax for configuration.

Getting the latest configuration
`````````````````````````````````
You may wish to consult the `config.toml in source control <https://raw.github.com/chobie/momonga/master/config.toml>`_ for all of the possible latest values.

General server
---------------

In the [server] section of config.toml, the following settings are tunable:

log_file
++++++++

like::

   log_file = "/var/log/momonga.log"

stdout, stderr are useful when debugging.

log_level
+++++++++

like::

   log_level = "debug"

debug, info, warn and error are available

pid_file
++++++++

like::

   pid_file = "/var/run/momonga.pid"

.. toctree::
   :maxdepth: 1

