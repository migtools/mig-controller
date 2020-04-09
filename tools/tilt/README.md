# Install Tilt

Installing the tilt binary is a one-step command:

```bash
curl -fsSL https://raw.githubusercontent.com/windmilleng/tilt/master/scripts/install.sh | bash
```

# Setup

1) `cp tilt-settings.json.example tilt-settings.json`
2) Edit `tilt-settings.json` and put your values for `image` and path to `migration_controller` template.
3) run `tilt up`

Alternatively you can run it from [Makefile](../../Makefile) by running `make tilt`
You can specify needed arguments with `make tilt IMG=* TEMPLATE=*`

# Why?

This tool automatically redeploys production `migration-controller` pod on any code change being detected at any stage of the development, simplifying testing and/or development process.

# Demo

![](./demo.gif)
