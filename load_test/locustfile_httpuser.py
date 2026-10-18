from locust import HttpUser, task, between


class ProductUser(HttpUser):
    wait_time = between(1, 2)

    @task(3)
    def get_product(self):
        self.client.get(
            "/v1/products/12345",
            timeout=10,
        )

    @task(1)
    def update_product(self):
        self.client.post(
            "/v1/products/12345/details",
            json={
                "product_id": 12345,
                "sku": "LOCUST-SKU-123",
                "manufacturer": "Locust Test Manufacturer",
                "category_id": 456,
                "weight": 100,
                "some_other_id": 789,
            },
            timeout=10,
        )
