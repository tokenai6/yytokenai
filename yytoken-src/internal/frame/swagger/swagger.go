package swagger

import (
	"github.com/gogf/gf/v2/net/ghttp"
)

// UITemplate 支持调试的Swagger UI模板
const UITemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="description" content="XWFrame API文档 - 支持在线调试"/>
    <title>XWFrame API文档</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.10.5/swagger-ui.min.css" />
    <style>
        html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin:0; background: #fafafa; }
        .swagger-ui .topbar { display: none; }
        .swagger-ui .info { margin: 20px 0; }
        .swagger-ui .scheme-container { background: #fff; border: 1px solid #e0e0e0; border-radius: 4px; }
        .swagger-ui .opblock { border-radius: 8px; margin-bottom: 16px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .swagger-ui .opblock-summary { border-radius: 8px 8px 0 0; }
        .swagger-ui .btn { border-radius: 6px; font-weight: 500; }
        .swagger-ui .btn.execute { background: #007bff; border-color: #007bff; }
        .swagger-ui .btn.execute:hover { background: #0056b3; border-color: #0056b3; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.29.1/swagger-ui-bundle.js" crossorigin></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/5.29.1/swagger-ui-standalone-preset.js" crossorigin></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: '{SwaggerUIDocUrl}',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                validatorUrl: null,
                tryItOutEnabled: true,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch'],
                docExpansion: 'list',
                defaultModelsExpandDepth: 3,
                defaultModelExpandDepth: 3,
                onComplete: function() {
                    console.log('Swagger UI loaded successfully');
                },
                requestInterceptor: function(request) {
                    const token = localStorage.getItem('swagger_token');
                    if (token) {
                        request.headers['Authorization'] = 'Bearer ' + token;
                    }
                    return request;
                },
                responseInterceptor: function(response) {
                    console.log('API Response:', response);
                    return response;
                }
            });

            const authButton = document.createElement('button');
            authButton.innerHTML = 'Set Token';
            authButton.style.cssText = 'position: fixed; top: 10px; right: 10px; z-index: 1000; padding: 10px 15px; background: #007bff; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; box-shadow: 0 2px 8px rgba(0,0,0,0.2);';
            authButton.onclick = function() {
                const token = prompt('Enter JWT Token:');
                if (token) {
                    localStorage.setItem('swagger_token', token);
                    alert('Token saved');
                }
            };
            document.body.appendChild(authButton);

            const clearButton = document.createElement('button');
            clearButton.innerHTML = 'Clear Token';
            clearButton.style.cssText = 'position: fixed; top: 50px; right: 10px; z-index: 1000; padding: 10px 15px; background: #dc3545; color: white; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; box-shadow: 0 2px 8px rgba(0,0,0,0.2);';
            clearButton.onclick = function() {
                localStorage.removeItem('swagger_token');
                alert('Token cleared');
            };
            document.body.appendChild(clearButton);
        };
    </script>
</body>
</html>
`

// InitSwagger 初始化Swagger配置
func InitSwagger(s *ghttp.Server) {
	s.SetSwaggerUITemplate(UITemplate)
}
