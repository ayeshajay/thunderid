// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import App from './App';
import withConfig from '@console/hocs/withConfig';
import withI18n from '@console/hocs/withI18n';
import withTheme from '@console/hocs/withTheme';

// The decorators are shell plumbing rather than a surface, so they are shared. Only the route tree
// is the Control Plane's own.
const AppWithDecorators = withConfig(withTheme(withI18n(App)));

export default AppWithDecorators;
